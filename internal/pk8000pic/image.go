package pk8000pic

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	_ "image/png"
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
	Graphics [0x1800]byte // массив графики: 3x2048 байт
	Colors   [0x1800]byte // массив цвета: 3x2048 байт
	Buffer   [0x300]byte  // буфер экрана: 3x256 байт (коды 00h..FFh)
}

// LoadImage декодирует PNG или GIF (первый кадр).
func LoadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, format, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("не удалось декодировать %s: %w", path, err)
	}
	_ = format
	if g, ok := img.(*image.Paletted); ok {
		_ = g
	}
	return img, nil
}

// loadGifFirstFrame нужен, чтобы image.Decode для GIF взял первый кадр.
func init() { _ = gif.DecodeAll }

// FitTo256x192 масштабирует изображение в 256x192.
// При stretch — с растяжением; иначе — с сохранением пропорций и чёрными полями.
func FitTo256x192(src image.Image, stretch bool) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, 256, 192))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if stretch {
		scaleInto(src, sb, dst, image.Rect(0, 0, 256, 192))
		return dst
	}
	// Вписать с сохранением пропорций.
	scale := math.Min(256.0/float64(sw), 192.0/float64(sh))
	dw := int(math.Round(float64(sw) * scale))
	dh := int(math.Round(float64(sh) * scale))
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	off := image.Rect((256-dw)/2, (192-dh)/2, (256-dw)/2+dw, (192-dh)/2+dh)
	scaleInto(src, sb, dst, off)
	return dst
}

// scaleInto — усреднение по области (box filter) из src в заданный прямоугольник dst.
func scaleInto(src image.Image, sb image.Rectangle, dst *image.RGBA, dr image.Rectangle) {
	for y := dr.Min.Y; y < dr.Max.Y; y++ {
		sy0 := sb.Min.Y + (y-dr.Min.Y)*sb.Dy()/dr.Dy()
		sy1 := sb.Min.Y + (y-dr.Min.Y+1)*sb.Dy()/dr.Dy()
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for x := dr.Min.X; x < dr.Max.X; x++ {
			sx0 := sb.Min.X + (x-dr.Min.X)*sb.Dx()/dr.Dx()
			sx1 := sb.Min.X + (x-dr.Min.X+1)*sb.Dx()/dr.Dx()
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var r, g, b, n uint32
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					pr, pg, pb, _ := src.At(sx, sy).RGBA()
					r += pr >> 8
					g += pg >> 8
					b += pb >> 8
					n++
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), 255})
		}
	}
}

func dist2(c color.RGBA, p color.RGBA) int {
	dr := int(c.R) - int(p.R)
	dg := int(c.G) - int(p.G)
	db := int(c.B) - int(p.B)
	return dr*dr + dg*dg + db*db
}

// EncodeScreen кодирует RGB-кадр 256x192 в структуры видео-ОЗУ режима 2.
// Для каждого отрезка из 8 точек подбираются цвета изображения/фона и битовая маска.
func EncodeScreen(img *image.RGBA) ScreenData {
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
			bestFG, bestBG := byte(0), byte(0)
			var bestBits byte
			bestErr := math.MaxInt
			for fg := 0; fg < 16; fg++ {
				for bg := 0; bg < 16; bg++ {
					var bits byte
					err := 0
					for i := 0; i < 8; i++ {
						e1 := dist2(palette[fg], pix[i])
						e2 := dist2(palette[bg], pix[i])
						if e1 < e2 {
							bits |= 0x80 >> i
							err += e1
						} else {
							err += e2
						}
					}
					if err < bestErr {
						bestErr = err
						bestFG, bestBG = byte(fg), byte(bg)
						bestBits = bits
					}
				}
			}
			part := y / 64
			chr := (y%64)/8*32 + cell
			idx := part*0x800 + chr*8 + y%8
			sd.Graphics[idx] = bestBits
			sd.Colors[idx] = bestFG | bestBG<<4
		}
	}
	return sd
}
