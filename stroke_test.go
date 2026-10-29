package rasterx_test

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"testing"

	"github.com/golang-gui/rasterx"
	"golang.org/x/image/math/fixed"
)

// 圆角折线等于各线段与半径为线宽一半的圆盘之和。
// 使用独立的点到线段距离验证轮廓，距边界 1.25 像素内留给抗锯齿。
func TestRoundStrokeOutline(t *testing.T) {
	paths := []struct {
		name   string
		points [][2]float64
	}{
		{"折返向右", [][2]float64{{40, 64}, {80, 64}, {40, 64}}},
		{"折返向左", [][2]float64{{80, 64}, {40, 64}, {80, 64}}},
		{"折返向下", [][2]float64{{64, 40}, {64, 80}, {64, 40}}},
		{"折返向上", [][2]float64{{64, 80}, {64, 40}, {64, 80}}},
		{"斜线折返", [][2]float64{{40, 40}, {80, 80}, {40, 40}}},
		{"近似折返正向", [][2]float64{{40, 64}, {80, 64}, {40, 64.125}}},
		{"近似折返反向", [][2]float64{{40, 64}, {80, 64}, {40, 63.875}}},
		{"同向连续", [][2]float64{{30, 64}, {60, 64}, {90, 64}}},
		{"直角", [][2]float64{{30, 64}, {64, 64}, {64, 30}}},
	}
	for _, path := range paths {
		for _, scale := range []int{1, 2, 4} {
			t.Run(path.name+"/"+strconv.Itoa(scale), func(t *testing.T) {
				img := image.NewRGBA(image.Rect(0, 0, 128*scale, 128*scale))
				r := rasterx.NewStroker(img.Bounds().Dx(), img.Bounds().Dy(), rasterx.NewScannerGV(img.Bounds().Dx(), img.Bounds().Dy(), img, img.Bounds()))
				r.SetStroke(12*64*fixed.Int26_6(scale), 4*64, rasterx.RoundCap, nil, nil, rasterx.Round)
				ink := color.RGBA{R: 64, G: 128, B: 192, A: 255}
				r.SetColor(ink)
				for i, p := range path.points {
					point := rasterx.ToFixedP(p[0]*float64(scale), p[1]*float64(scale))
					if i == 0 {
						r.Start(point)
					} else {
						r.Line(point)
					}
				}
				r.Stop(false)
				r.Draw()
				for y := 0; y < img.Bounds().Dy(); y++ {
					for x := 0; x < img.Bounds().Dx(); x++ {
						distance := math.Inf(1)
						for i := 1; i < len(path.points); i++ {
							a, b := path.points[i-1], path.points[i]
							dx, dy := b[0]-a[0], b[1]-a[1]
							px, py := (float64(x)+.5)/float64(scale)-a[0], (float64(y)+.5)/float64(scale)-a[1]
							u := math.Max(0, math.Min(1, (px*dx+py*dy)/(dx*dx+dy*dy)))
							distance = math.Min(distance, math.Hypot(px-u*dx, py-u*dy)*float64(scale))
						}
						got := img.RGBAAt(x, y)
						if distance < 6*float64(scale)-1.25 && got != ink {
							t.Fatalf("内部像素 (%d,%d) = %v，期望 %v；距中心线 %.3f", x, y, got, ink, distance)
						}
						if distance > 6*float64(scale)+1.25 && got.A != 0 {
							t.Fatalf("轮廓外像素 (%d,%d) = %v，距中心线 %.3f", x, y, got, distance)
						}
					}
				}
			})
		}
	}
}

// 二次曲线的最远点为 40+(120-40)/2=80；三次曲线为 40+3*(100-40)/4=85。
// 曲线内部的折返没有命令连接点，其轮廓不应受 stroke-linejoin 影响。
func TestCurveReversalOutline(t *testing.T) {
	for _, curve := range []struct {
		name string
		endX int
		draw func(*rasterx.Stroker)
	}{
		{"quadratic", 80, func(r *rasterx.Stroker) { r.QuadBezier(rasterx.ToFixedP(120, 64), rasterx.ToFixedP(40, 64)) }},
		{"cubic", 85, func(r *rasterx.Stroker) {
			r.CubeBezier(rasterx.ToFixedP(100, 64), rasterx.ToFixedP(100, 64), rasterx.ToFixedP(40, 64))
		}},
		{"cubic-two-cusps", 63, func(r *rasterx.Stroker) {
			r.CubeBezier(rasterx.ToFixedP(120, 64), rasterx.ToFixedP(-40, 64), rasterx.ToFixedP(40, 64))
		}},
	} {
		for _, join := range []rasterx.JoinMode{rasterx.Round, rasterx.Bevel, rasterx.Miter} {
			t.Run(curve.name+"/"+strconv.Itoa(int(join)), func(t *testing.T) {
				img := image.NewRGBA(image.Rect(0, 0, 128, 128))
				r := rasterx.NewStroker(128, 128, rasterx.NewScannerGV(128, 128, img, img.Bounds()))
				r.SetStroke(12*64, 4*64, rasterx.RoundCap, nil, nil, join)
				r.SetColor(color.RGBA{G: 255, A: 255})
				r.Start(rasterx.ToFixedP(40, 64))
				curve.draw(r)
				r.Stop(false)
				r.Draw()
				if got := img.RGBAAt(curve.endX+4, 64); got != (color.RGBA{G: 255, A: 255}) {
					t.Fatalf("曲线折返处内部像素 = %v，期望绿色", got)
				}
				if got := img.RGBAAt(curve.endX+8, 64); got.A != 0 {
					t.Fatalf("曲线折返处外部像素 = %v，期望透明", got)
				}
			})
		}
	}
}
