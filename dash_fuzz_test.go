//go:build go1.18

package rasterx_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/golang-gui/rasterx"
)

// 有界的有效输入覆盖零间隔、负偏移、重复点和多次方向变化，验证无 panic 和非法预乘像素。
func FuzzDashSegments(f *testing.F) {
	f.Add([]byte{0, 10, 0, 0, 3, 4, 4, 4, 4, 3, 0, 0})
	f.Add([]byte{8, 0, 5, 1, 0, 0, 1, 7, 7, 1, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 4 || len(data) > 48 {
			return
		}
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		r := rasterx.NewDasher(64, 64, rasterx.NewScannerGV(64, 64, img, img.Bounds()))
		cap := rasterx.RoundCap
		if data[3]&1 != 0 {
			cap = rasterx.SquareCap
		}
		r.SetStroke(4*64, 4*64, cap, nil, nil, rasterx.Miter, []float64{float64(data[0] % 12), float64(data[1] % 12)}, float64(int8(data[2])))
		r.SetColor(color.RGBA{R: 23, G: 91, B: 141, A: 192})
		x, y := 32.0, 32.0
		r.Start(rasterx.ToFixedP(x, y))
		for i := 4; i+1 < len(data); i += 2 {
			x += float64(int(data[i]%9) - 4)
			y += float64(int(data[i+1]%9) - 4)
			r.Line(rasterx.ToFixedP(x, y))
		}
		r.Stop(false)
		r.Draw()
		for i := 0; i < len(img.Pix); i += 4 {
			a := img.Pix[i+3]
			if img.Pix[i] > a || img.Pix[i+1] > a || img.Pix[i+2] > a {
				t.Fatal("出现非法预乘像素")
			}
		}
	})
}
