package rasterx_test

import (
	"image"
	"math"
	"testing"

	"github.com/golang-gui/rasterx"
	"golang.org/x/image/math/fixed"
)

// 向最近的 1/64 单位取整，正负坐标对称；整数附近的浮点误差不得丢失整格。
func TestFixedPointRounding(t *testing.T) {
	for _, tt := range []struct {
		value float64
		want  fixed.Int26_6
	}{
		{math.Nextafter(16, 0), 1024},
		{math.Nextafter(-16, 0), -1024},
		{.01, 1}, {-.01, -1}, {.007, 0}, {-.007, 0},
	} {
		if got := rasterx.ToFixedP(tt.value, tt.value); got.X != tt.want || got.Y != tt.want {
			t.Errorf("ToFixedP(%g)=%v，期望 %v", tt.value, got, tt.want)
		}
		m := rasterx.Identity.Translate(tt.value, tt.value)
		if got := m.TFixed(fixed.Point26_6{}); got.X != tt.want || got.Y != tt.want {
			t.Errorf("TFixed(%g)=%v，期望 %v", tt.value, got, tt.want)
		}
	}
}

// 所有控制点均在 y=64 的曲线，任何离散顶点都应保持 y=64。
func TestBezierSamplesOnStraightAxis(t *testing.T) {
	for _, cubic := range []bool{false, true} {
		img := image.NewRGBA(image.Rect(0, 0, 128, 128))
		s := &joinOutlineRecorder{Scanner: rasterx.NewScannerGV(128, 128, img, img.Bounds())}
		r := rasterx.NewFiller(128, 128, s)
		r.Start(rasterx.ToFixedP(16, 64))
		if cubic {
			r.CubeBezier(rasterx.ToFixedP(100, 64), rasterx.ToFixedP(100, 64), rasterx.ToFixedP(16, 64))
		} else {
			r.QuadBezier(rasterx.ToFixedP(100, 64), rasterx.ToFixedP(16, 64))
		}
		for _, p := range s.points {
			if p.Y != 64*64 {
				t.Fatalf("cubic=%v，平直曲线顶点 y=%v，期望 64", cubic, p.Y)
			}
		}
	}
}
