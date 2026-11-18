package pk8000anim

import (
	"image"
	"image/color"
	"testing"
)

func TestBlitModes(t *testing.T) {
	// кадр меньше экрана — левый верхний угол, 1:1
	small := image.NewRGBA(image.Rect(0, 0, 100, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 100; x++ {
			small.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	out := BlitTo256x192(small)
	if out.RGBAAt(0, 0).R != 255 || out.RGBAAt(99, 59).R != 255 {
		t.Errorf("малый кадр должен быть в левом верхнем углу 1:1")
	}
	if out.RGBAAt(100, 0).R != 0 || out.RGBAAt(0, 60).R != 0 || out.RGBAAt(255, 191).R != 0 {
		t.Errorf("вне кадра должно быть чёрное поле")
	}

	// кадр больше экрана — обрезается
	big := image.NewRGBA(image.Rect(0, 0, 300, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 300; x++ {
			big.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	out = BlitTo256x192(big)
	if out.RGBAAt(255, 191).R != 255 {
		t.Errorf("область 256x192 должна быть заполнена")
	}
	// пиксель за границей экрана исходника (x=256) не должен попадать в кадр
	big.SetRGBA(256, 0, color.RGBA{0, 255, 0, 255})
	out = BlitTo256x192(big)
	if out.RGBAAt(0, 0).G != 255 {
		t.Errorf("кадр должен переноситься с левого верхнего пикселя 1:1")
	}
}
