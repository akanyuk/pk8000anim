package pk8000anim

import (
	"fmt"
	"sort"
)

// Поток memsave для ПК8000 (адреса — физические в видео-ОЗУ 0000h..37FFh):
//
//	dw addr      — начальный адрес указателя экрана
//	%00nnnnnn    — COPY  nnnnnn+1 (1..64) байт из потока в видео-ОЗУ
//	%01nnnnnn    — SKIP  nnnnnn+1 (1..64) байт указателя экрана
//	%100nnnnn    — FILL  nnnnnn+1 (1..32) байта следующим байтом потока
//	%101yxxxx    — LONG SKIP: +xxxx*256 (+128, если y=1)
//	%11xxxxxx    — END кадра
const (
	cmdEnd = 0xC0
)

// Diff — один кадр-дифф анимации (переход f_i -> f_{i+1}) и задержка f_i.
type Diff struct {
	Delay  int
	Stream []byte
}

// EncodeDiffs строит потоки переходов f0->f1, ..., f(N-2)->f(N-1), f(N-1)->f0
// (последний — замыкание цикла: после него состояние равно ключевому кадру).
// delays[i] — длительность показа кадра i в тиках (1/100 с).
func EncodeDiffs(frames []ScreenData, delays []int, bw bool) []Diff {
	n := len(frames)
	diffs := make([]Diff, 0, n)
	for i := 0; i < n; i++ {
		prev := frames[i]
		cur := frames[(i+1)%n]
		diffs = append(diffs, Diff{
			Delay:  delays[i],
			Stream: encodeStream(changedBytes(prev, cur, bw), bw),
		})
	}
	return diffs
}

func encodeStream(changes map[int]byte, bw bool) []byte {
	addrs := make([]int, 0, len(changes))
	for a := range changes {
		addrs = append(addrs, a)
	}
	sort.Ints(addrs)

	out := []byte{0, 0} // dw начального адреса (заполним ниже)
	if len(addrs) == 0 {
		return append(out, cmdEnd)
	}
	out[0] = byte(addrs[0])
	out[1] = byte(addrs[0] >> 8)

	ptr := addrs[0]
	i := 0
	for i < len(addrs) {
		// собрать непрерывный кусок изменённых адресов
		j := i
		for j+1 < len(addrs) && addrs[j+1] == addrs[j]+1 {
			j++
		}
		// пропуск до начала куска
		out = appendSkip(out, addrs[i]-ptr)
		ptr = addrs[i]
		// данные куска с FILL-оптимизацией
		k := i
		for k <= j {
			// гряда одинаковых байт -> FILL (от 2 штук выгодно)
			run := 1
			for k+run <= j && changes[addrs[k+run]] == changes[addrs[k]] {
				run++
			}
			if run >= 2 {
				for r := 0; r < run; {
					cnt := run - r
					if cnt > 32 {
						cnt = 32
					}
					out = append(out, 0x80|byte(cnt-1), changes[addrs[k]])
					r += cnt
				}
				k += run
				ptr += run
				continue
			}
			// одиночные байты -> COPY (до 64)
			start := k
			for k <= j {
				nrun := 1
				for k+nrun <= j && changes[addrs[k+nrun]] == changes[addrs[k]] {
					nrun++
				}
				if nrun >= 2 {
					break
				}
				k++
				if k-start == 64 {
					break
				}
			}
			cnt := k - start
			out = append(out, byte(cnt-1)) // %00nnnnnn
			for a := start; a < k; a++ {
				out = append(out, changes[addrs[a]])
			}
			ptr += cnt
		}
		i = j + 1
	}
	return append(out, cmdEnd)
}

// appendSkip кодирует пропуск d байт (d >= 0) командами SKIP / LONG SKIP.
func appendSkip(out []byte, d int) []byte {
	if d <= 0 {
		return out
	}
	cur := 0
	for d-cur > 128 {
		delta := (d - cur) / 256
		if delta > 15 {
			delta = 15
		}
		cur += delta * 256
		cmd := byte(0xA0) | byte(delta) // %1010xxxx
		if d-cur >= 128 {
			cmd |= 0x10 // y=1: ещё +128
			cur += 128
		}
		out = append(out, cmd)
	}
	for cur < d {
		step := d - cur
		if step > 64 {
			step = 64
		}
		out = append(out, 0x40|byte(step-1)) // %01nnnnnn
		cur += step
	}
	return out
}

// changedBytes — байты, отличающиеся между кадрами, по физическим адресам ВОЗУ:
// графика 0000h-17FFh и (кроме bw) цвет 2000h-37FFh. Буфер экрана константен.
func changedBytes(prev, cur ScreenData, bw bool) map[int]byte {
	changes := map[int]byte{}
	for i := 0; i < 0x1800; i++ {
		if prev.Graphics[i] != cur.Graphics[i] {
			changes[i] = cur.Graphics[i]
		}
		if !bw && prev.Colors[i] != cur.Colors[i] {
			changes[0x2000+i] = cur.Colors[i]
		}
	}
	return changes
}

// SelfCheck проигрывает потоки поверх ключевого кадра и сверяет результат
// с ожидаемыми кадрами (включая замыкание цикла на кадр 0).
func SelfCheck(frames []ScreenData, diffs []Diff, bw bool) error {
	n := len(frames)
	if n == 0 {
		return fmt.Errorf("SelfCheck: нет кадров")
	}
	if len(diffs) != n {
		return fmt.Errorf("SelfCheck: диффов %d, ожидалось %d", len(diffs), n)
	}
	var vram [0x4000]byte
	copy(vram[0x0000:0x1800], frames[0].Graphics[:])
	copy(vram[0x1800:0x1B00], frames[0].Buffer[:])
	copy(vram[0x2000:0x3800], frames[0].Colors[:])

	for i, d := range diffs {
		if err := applyStream(vram[:], d.Stream); err != nil {
			return fmt.Errorf("SelfCheck: дифф %d: %w", i, err)
		}
		want := frames[(i+1)%n]
		if err := compareFrame(vram[:], want, bw); err != nil {
			return fmt.Errorf("SelfCheck: дифф %d (кадр %d -> %d): %w", i, i, (i+1)%n, err)
		}
	}
	return nil
}

// applyStream воспроизводит memsave-поток (как плеер) в буфер видео-ОЗУ.
func applyStream(vram []byte, stream []byte) error {
	if len(stream) < 3 {
		return fmt.Errorf("слишком короткий поток")
	}
	ptr := int(stream[0]) | int(stream[1])<<8
	i := 2
	for {
		if i >= len(stream) {
			return fmt.Errorf("нет команды END")
		}
		cmd := stream[i]
		i++
		switch {
		case cmd&0xC0 == 0x00: // COPY
			cnt := int(cmd&0x3F) + 1
			if i+cnt > len(stream) {
				return fmt.Errorf("COPY выходит за поток")
			}
			for k := 0; k < cnt; k++ {
				if ptr >= 0x4000 {
					return fmt.Errorf("COPY: указатель %04Xh за границей", ptr)
				}
				vram[ptr] = stream[i]
				i++
				ptr++
			}
		case cmd&0xC0 == 0x40: // SKIP
			ptr += int(cmd&0x3F) + 1
		case cmd&0xC0 == 0x80 && cmd&0x20 == 0: // FILL
			cnt := int(cmd&0x1F) + 1
			if i >= len(stream) {
				return fmt.Errorf("FILL: нет байта значения")
			}
			val := stream[i]
			i++
			for k := 0; k < cnt; k++ {
				if ptr >= 0x4000 {
					return fmt.Errorf("FILL: указатель %04Xh за границей", ptr)
				}
				vram[ptr] = val
				ptr++
			}
		case cmd&0xC0 == 0x80: // LONG SKIP
			ptr += int(cmd&0x0F) * 256
			if cmd&0x10 != 0 {
				ptr += 128
			}
		default: // END
			return nil
		}
	}
}

func compareFrame(vram []byte, want ScreenData, bw bool) error {
	for i := 0; i < 0x1800; i++ {
		if vram[i] != want.Graphics[i] {
			return fmt.Errorf("графика[%04Xh]: %02Xh != %02Xh", i, vram[i], want.Graphics[i])
		}
		if !bw && vram[0x2000+i] != want.Colors[i] {
			return fmt.Errorf("цвет[%04Xh]: %02Xh != %02Xh", i, vram[0x2000+i], want.Colors[i])
		}
	}
	return nil
}
