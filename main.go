package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/akanyuk/sura_ap/internal/pk8000anim"
)

const usage = `pk8000anim — парсер GIF-анимации в программы ПК8000 (Хобби, Веста, Сура).

Использование:
  pk8000anim parse <анимация.gif> [флаги]
      Генерирует проект анимации: player.asm (плеер), data.asm (данные),
      build.sh/build.bat и собранный .cas.
      Флаги:
        -o каталог     каталог вывода (по умолчанию рядом с GIF)
        -name имя      имя проекта/файла на ленте (до 6 символов)
        -border N      цвет рамки 0..15 (по умолчанию 0 — чёрный)
        -stretch       растянуть кадры до 256x192 без сохранения пропорций
        -bw            принудительно чёрно-белый режим парсинга
                       (автоматически включается для двухцветных GIF)

  pk8000anim asm <player.asm> <data.asm> [файлы...] [флаги]
      Собирает исходники ассемблера 8080 в .cas (файлы объединяются по порядку).
      Флаги:
        -o файл.cas    имя выходного файла
        -name имя      имя файла на магнитной ленте (до 6 символов)

Примеры:
  pk8000anim parse anim.gif -o out
  pk8000anim parse anim.gif -bw -border 7
  pk8000anim asm out/player.asm out/data.asm -o out/anim.cas
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "parse":
		err = cmdParse(os.Args[2:])
	case "asm":
		err = cmdAsm(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		// `pk8000anim file.gif` — сокращение для parse
		if strings.EqualFold(filepath.Ext(os.Args[1]), ".gif") {
			err = cmdParse(os.Args[1:])
		} else {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}

func cmdParse(args []string) error {
	args = reorderArgs(args, map[string]bool{"o": true, "name": true, "border": true})
	fs := flag.NewFlagSet("parse", flag.ExitOnError)
	outDir := fs.String("o", "", "каталог вывода")
	name := fs.String("name", "", "имя проекта")
	border := fs.Int("border", 0, "цвет рамки 0..15")
	stretch := fs.Bool("stretch", false, "растянуть до 256x192")
	bw := fs.Bool("bw", false, "чёрно-белый режим парсинга")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return fmt.Errorf("укажите ровно один GIF-файл")
	}
	if *border < 0 || *border > 15 {
		return fmt.Errorf("цвет рамки должен быть в диапазоне 0..15")
	}
	gifPath := fs.Arg(0)
	if ext := strings.ToLower(filepath.Ext(gifPath)); ext != ".gif" {
		if ext == ".png" {
			return fmt.Errorf("PNG больше не поддерживается — используйте GIF")
		}
		return fmt.Errorf("поддерживаются только GIF-файлы (получен %q)", ext)
	}

	proj := *name
	if proj == "" {
		proj = strings.TrimSuffix(filepath.Base(gifPath), filepath.Ext(gifPath))
	}
	safe := sanitizeName(proj)
	dir := *outDir
	if dir == "" {
		dir = filepath.Dir(gifPath)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	frames, err := pk8000anim.LoadGIF(gifPath, *stretch)
	if err != nil {
		return err
	}

	// авто-ч/б: двухцветный «чёрный по белому» GIF
	imgs := make([]*image.RGBA, 0, len(frames))
	for _, f := range frames {
		imgs = append(imgs, f.Img)
	}
	mono := *bw || pk8000anim.IsBWShot(imgs)
	kind := "цветной"
	if mono {
		kind = "чёрно-белый"
	}
	fmt.Printf("GIF: %s\nКадров: %d, режим парсинга: %s\n", gifPath, len(frames), kind)

	screen := make([]pk8000anim.ScreenData, 0, len(frames))
	delays := make([]int, 0, len(frames))
	for _, f := range frames {
		screen = append(screen, pk8000anim.EncodeScreen(f.Img, mono))
		delays = append(delays, f.Delay)
	}

	diffs := pk8000anim.EncodeDiffs(screen, delays, mono)
	if err := pk8000anim.SelfCheck(screen, diffs, mono); err != nil {
		return fmt.Errorf("самопроверка потоков: %w", err)
	}
	streamBytes := 0
	for _, d := range diffs {
		streamBytes += len(d.Stream)
	}
	fmt.Printf("Дифф-кадров: %d, поток: %d байт\n", len(diffs), streamBytes)

	playerSrc := pk8000anim.EmitPlayerASM(byte(*border), mono)
	dataSrc := pk8000anim.EmitDataASM(screen[0], diffs, mono)
	playerPath := filepath.Join(dir, "player.asm")
	dataPath := filepath.Join(dir, "data.asm")
	if err := os.WriteFile(playerPath, []byte(playerSrc), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(dataPath, []byte(dataSrc), 0o644); err != nil {
		return err
	}
	fmt.Printf("Исходники: %s, %s\n", playerPath, dataPath)

	shellCmd, batCmd := pk8000anim.AsmInvocation()
	casName := safe + ".cas"
	sh, bat := pk8000anim.EmitBuildScripts(casName, shellCmd, batCmd)
	if err := os.WriteFile(filepath.Join(dir, "build.sh"), []byte(sh), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "build.bat"), []byte(bat), 0o644); err != nil {
		return err
	}
	fmt.Printf("Скрипты сборки: %s, %s\n",
		filepath.Join(dir, "build.sh"), filepath.Join(dir, "build.bat"))

	_, code, err := pk8000anim.Assemble(playerSrc + "\n" + dataSrc)
	if err != nil {
		return fmt.Errorf("ассемблирование: %w", err)
	}
	if err := checkBudget(code); err != nil {
		return err
	}
	cas := pk8000anim.BuildCAS(safe, 0x4000, 0x4000, code)
	if err := os.WriteFile(filepath.Join(dir, casName), cas, 0o644); err != nil {
		return err
	}
	fmt.Printf("Готовая анимация: %s (%d байт по адресу 4000h)\n", filepath.Join(dir, casName), len(code))
	return nil
}

func cmdAsm(args []string) error {
	args = reorderArgs(args, map[string]bool{"o": true, "name": true})
	fs := flag.NewFlagSet("asm", flag.ExitOnError)
	out := fs.String("o", "", "выходной .cas")
	name := fs.String("name", "", "имя файла на ленте")
	fs.Parse(args)
	if fs.NArg() < 1 {
		return fmt.Errorf("укажите исходные файлы .asm")
	}
	asmPaths := fs.Args()
	outPath := *out
	if outPath == "" {
		outPath = strings.TrimSuffix(asmPaths[0], filepath.Ext(asmPaths[0])) + ".cas"
	}
	proj := *name
	if proj == "" {
		proj = filepath.Base(strings.TrimSuffix(asmPaths[0], filepath.Ext(asmPaths[0])))
	}

	var srcs []string
	for _, p := range asmPaths {
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		srcs = append(srcs, string(b))
	}
	_, code, err := pk8000anim.Assemble(strings.Join(srcs, "\n"))
	if err != nil {
		return fmt.Errorf("%s: %w", asmPaths[0], err)
	}
	if err := checkBudget(code); err != nil {
		return err
	}
	cas := pk8000anim.BuildCAS(proj, 0x4000, 0x4000, code)
	if err := os.WriteFile(outPath, cas, 0o644); err != nil {
		return err
	}
	fmt.Printf("Ассемблировано: %d байт по адресу 4000h\nГотово: %s\n", len(code), outPath)
	return nil
}

// checkBudget: программа с данными должна помещаться в 4000h..EFFFh
// (F000h-FFFFh заняты системной областью).
func checkBudget(code []byte) error {
	const maxEnd = 0xF000
	end := 0x4000 + len(code)
	if end > maxEnd {
		return fmt.Errorf("анимация не помещается: %d байт (предел %d, лишних %d)",
			len(code), maxEnd-0x4000, end-maxEnd)
	}
	if end > 0x4000+(maxEnd-0x4000)*9/10 {
		fmt.Printf("Внимание: занято %d из %d байт памяти под программу\n", len(code), maxEnd-0x4000)
	}
	return nil
}

// reorderArgs выносит позиционные аргументы в конец, чтобы флаги можно было
// указывать в любом порядке: `asm a.asm b.asm -o x.cas` -> `-o x.cas a.asm b.asm`.
// valueFlags — имена флагов, принимающих значение.
func reorderArgs(args []string, valueFlags map[string]bool) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if strings.Contains(name, "=") {
				continue
			}
			if valueFlags[name] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	return append(flags, pos...)
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '.' {
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		out = "anim"
	}
	return out
}
