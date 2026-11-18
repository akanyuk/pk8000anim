package pk8000anim

import (
	"image"
	"image/color"
	"testing"
)

func TestFitModes(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 100; x++ {
			src.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}

	fit := FitTo256x192(src, false)
	// ожидание: scale=0.96 -> 96x192 по центру: x=80..175 белый, остальное чёрное
	if fit.RGBAAt(40, 96).R != 0 {
		t.Errorf("fit: слева должен быть чёрное поле, получено %v", fit.RGBAAt(40, 96))
	}
	if fit.RGBAAt(120, 96).R != 255 {
		t.Errorf("fit: центр должен быть белым, получено %v", fit.RGBAAt(120, 96))
	}
	if fit.RGBAAt(220, 96).R != 0 {
		t.Errorf("fit: справа должно быть чёрное поле, получено %v", fit.RGBAAt(220, 96))
	}

	st := FitTo256x192(src, true)
	if st.RGBAAt(5, 5).R != 255 || st.RGBAAt(250, 186).R != 255 {
		t.Errorf("stretch: изображение должно заполнять весь кадр")
	}
}
