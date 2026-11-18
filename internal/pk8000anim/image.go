package pk8000anim

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"math"
	"os"
)

// Палитра ПК8000 (соответствует ПЗУ/эмулятору): 0 и 1 — чёрный.
var palette = [16]color.RGBA{
	{0x00, 0x00, 0x00, 255}, // 0  чёрный
	{0x00, 0x00, 0x00, 255}, // 1  чёрный
	{0x00, 0xA8, 0x00, 255}, // 2  зелёный
	{0x00, 0xFF, 0x00, 255}, // 3  светло-зелёный
	{0x00, 0x00, 0xA8, 255}, // 4  синий
	{0x00, 0x00, 0xFF, 255}, // 5  светло-синий
	{0x00, 0xA8, 0xA8, 255}, // 6  бирюзовый
	{0x00, 0xFF, 0xFF, 255}, // 7  светло-голубой
	{0xA8, 0x00, 0x00, 255}, // 8  красный
	{0xFF, 0x00, 0x00, 255}, // 9  светло-красный
	{0xA8, 0xA8, 0x00, 255}, // 10 жёлтый
	{0xFF, 0xFF, 0x00, 255}, // 11 светло-жёлтый
	{0xA8, 0x00, 0xA8, 255}, // 12 фиолетовый
	{0xFF, 0x00, 0xFF, 255}, // 13 светло-фиолетовый
	{0xA8, 0xA8, 0xA8, 255}, // 14 серый
	{0xFF, 0xFF, 0xFF, 255}, // 15 белый
}

// ScreenData — кадровый буфер режима 2 ПК8000.
type ScreenData struct {
	Graphics [0x1800]byte // массив графики: 3x2048 байт (0000h-17FFh)
	Colors   [0x1800]byte // массив цвета: 3x2048 байт (2000h-37FFh)
	Buffer   [0x300]byte  // буфер экрана: 3x256 кодов (1800h-1AFFh), константа
}

// Frame — кадр GIF после композиции и масштабирования в 256x192.
type Frame struct {
	Img   *image.RGBA
	Delay int // длительность показа, тиков (1 тик = 1/100 с)
}

// LoadGIF читает GIF-анимацию: композиция кадров (disposal, смещения, прозрачность),
// задержки кадров (0 -> 1 тик, максимум 255). Каждый кадр переносится в экран
// 256x192 попиксельно (1:1) в левый верхний угол, лишнее обрезается.
// Вторым значением возвращает признак чёрно-белого GIF.
func LoadGIF(path string) ([]Frame, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil, false, fmt.Errorf("не удалось декодировать %s: %w", path, err)
	}
	if len(g.Image) == 0 {
		return nil, false, fmt.Errorf("%s: в GIF нет кадров", path)
	}

	bg := color.RGBA{0, 0, 0, 255}
	if pal, ok := g.Config.ColorModel.(color.Palette); ok && int(g.BackgroundIndex) < len(pal) {
		if c, ok := pal[g.BackgroundIndex].(color.RGBA); ok {
			bg = c
		} else if c, ok := pal[g.BackgroundIndex].(color.NRGBA); ok {
			bg = color.RGBA{c.R, c.G, c.B, 255}
		}
	}

	w, h := g.Config.Width, g.Config.Height
	if w == 0 || h == 0 {
		b := g.Image[0].Bounds()
		w, h = b.Max.X, b.Max.Y
	}
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))

	colors := map[color.RGBA]bool{}
	var frames []Frame
	for i, src := range g.Image {
		var snapshot *image.RGBA
		disposal := byte(0)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		if disposal == gif.DisposalPrevious {
			snapshot = image.NewRGBA(canvas.Rect)
			draw.Draw(snapshot, canvas.Rect, canvas, image.Point{}, draw.Src)
		}

		draw.Draw(canvas, src.Bounds(), src, src.Bounds().Min, draw.Over)

		if len(colors) <= 2 {
			b := canvas.Bounds()
		loop:
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					colors[canvas.RGBAAt(x, y)] = true
					if len(colors) > 2 {
						break loop
					}
				}
			}
		}

		delay := 1
		if i < len(g.Delay) {
			delay = g.Delay[i]
		}
		if delay <= 0 {
			delay = 1
		}
		if delay > 255 {
			delay = 255
		}
		frames = append(frames, Frame{Img: BlitTo256x192(canvas), Delay: delay})

		switch disposal {
		case gif.DisposalBackground:
			draw.Draw(canvas, src.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			if snapshot != nil {
				draw.Draw(canvas, canvas.Rect, snapshot, image.Point{}, draw.Src)
			}
		}
	}
	return frames, isBWColors(colors), nil
}

// isBWColors: не более двух цветов, из которых один тёмный и один светлый.
func isBWColors(colors map[color.RGBA]bool) bool {
	if len(colors) > 2 {
		return false
	}
	var minL, maxL = 255, 0
	for c := range colors {
		l := luminance(c)
		if l < minL {
			minL = l
		}
		if l > maxL {
			maxL = l
		}
	}
	return minL < 64 && maxL >= 160
}

func luminance(c color.RGBA) int {
	return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000
}

// BlitTo256x192 переносит кадр в экран 256x192 попиксельно (1:1).
// Кадр меньше экрана — в левый верхний угол, больше — обрезается.
func BlitTo256x192(src image.Image) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, 256, 192))
	sb := src.Bounds()
	w, h := sb.Dx(), sb.Dy()
	if w > 256 {
		w = 256
	}
	if h > 192 {
		h = 192
	}
	draw.Draw(dst, image.Rect(0, 0, w, h), src, sb.Min, draw.Src)
	return dst
}

func dist2(c color.RGBA, p color.RGBA) int {
	dr := int(c.R) - int(p.R)
	dg := int(c.G) - int(p.G)
	db := int(c.B) - int(p.B)
	return dr*dr + dg*dg + db*db
}

// EncodeScreen кодирует RGB-кадр 256x192 в структуры видео-ОЗУ режима 2.
// В режиме bw — только чёрный/белый по яркости; массив цвета константен (0Fh).
func EncodeScreen(img *image.RGBA, bw bool) ScreenData {
	var sd ScreenData
	for part := 0; part < 3; part++ {
		for row := 0; row < 8; row++ {
			for col := 0; col < 32; col++ {
				chr := byte(row*32 + col)
				sd.Buffer[part*0x100+int(chr)] = chr
			}
		}
	}
	for y := 0; y < 192; y++ {
		for cell := 0; cell < 32; cell++ {
			var pix [8]color.RGBA
			for i := 0; i < 8; i++ {
				pix[i] = img.RGBAAt(cell*8+i, y)
			}

			var bits byte
			var colByte byte
			if bw {
				for i := 0; i < 8; i++ {
					if luminance(pix[i]) >= 128 {
						bits |= 0x80 >> i
					}
				}
				colByte = 0x0F // fg = белый (15), bg = чёрный (0)
			} else {
				bestFG, bestBG := byte(0), byte(0)
				bestErr := math.MaxInt
				for fg := 0; fg < 16; fg++ {
					for bg := 0; bg < 16; bg++ {
						var b byte
						err := 0
						for i := 0; i < 8; i++ {
							e1 := dist2(palette[fg], pix[i])
							e2 := dist2(palette[bg], pix[i])
							if e1 < e2 {
								b |= 0x80 >> i
								err += e1
							} else {
								err += e2
							}
						}
						if err < bestErr {
							bestErr = err
							bestFG, bestBG = byte(fg), byte(bg)
							bits = b
						}
					}
				}
				colByte = bestFG | bestBG<<4
			}

			part := y / 64
			chr := (y%64)/8*32 + cell
			idx := part*0x800 + chr*8 + y%8
			sd.Graphics[idx] = bits
			sd.Colors[idx] = colByte
		}
	}
	return sd
}
