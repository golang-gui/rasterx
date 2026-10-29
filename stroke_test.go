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

type joinOutlineRecorder struct {
	rasterx.Scanner
	points []fixed.Point26_6
}

func (s *joinOutlineRecorder) Start(p fixed.Point26_6) {
	s.points = append(s.points, p)
	s.Scanner.Start(p)
}
func (s *joinOutlineRecorder) Line(p fixed.Point26_6) {
	s.points = append(s.points, p)
	s.Scanner.Line(p)
}

// 3-4-5 法向量无需归一化误差；偏移直线交点为 (15,-5)，裁切平面距顶点 10。
func TestMiterClipPlane(t *testing.T) {
	for _, mode := range []rasterx.JoinMode{rasterx.MiterClip, rasterx.ArcClip} {
		t.Run(strconv.Itoa(int(mode)), func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, 128, 128))
			s := &joinOutlineRecorder{Scanner: rasterx.NewScannerGV(128, 128, img, img.Bounds())}
			r := rasterx.NewStroker(128, 128, s)
			r.SetStroke(10*64, 2*64, nil, nil, nil, mode)
			r.Joiner(rasterx.C2Point{P: rasterx.ToFixedP(64, 64), TTan: rasterx.ToFixedP(4, 0), LTan: rasterx.ToFixedP(-4, 3), TNorm: rasterx.ToFixedP(0, -5), LNorm: rasterx.ToFixedP(3, 4)})
			length := math.Sqrt(250)
			projection := 25 / length
			fraction := (10 - projection) / (length - projection)
			for _, normal := range [][2]float64{{0, -5}, {3, 4}} {
				x := 64 + normal[0] + fraction*(15-normal[0])
				y := 64 + normal[1] + fraction*(-5-normal[1])
				nearest := math.Inf(1)
				for _, p := range s.points {
					nearest = math.Min(nearest, math.Hypot(float64(p.X)/64-x, float64(p.Y)/64-y))
				}
				if nearest > 1.0/64 {
					t.Errorf("裁切交点 (%.5f,%.5f) 未出现，最近轮廓点距离 %.5f，允许 1/64", x, y, nearest)
				}
			}
		})
	}
}

// 重合的端点控制点会产生多个量化后相同的采样点；圆端帽的内部不能因此变透明。
func TestBezierEndpointCoverage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	r := rasterx.NewStroker(128, 128, rasterx.NewScannerGV(128, 128, img, img.Bounds()))
	r.SetStroke(12*64, 4*64, rasterx.RoundCap, nil, nil, rasterx.Round)
	r.SetColor(color.RGBA{A: 255})
	r.Start(rasterx.ToFixedP(40, 40))
	r.CubeBezier(rasterx.ToFixedP(40, 40), rasterx.ToFixedP(80, 70), rasterx.ToFixedP(80, 100))
	r.Stop(false)
	r.Draw()
	for y := 35; y < 45; y++ {
		for x := 35; x < 45; x++ {
			if math.Hypot(float64(x)+.5-40, float64(y)+.5-40) < 4.5 && img.RGBAAt(x, y).A != 255 {
				t.Fatalf("端帽内部 (%d,%d) alpha=%d，期望 255", x, y, img.RGBAAt(x, y).A)
			}
		}
	}
}

// 自定义 gap 仍需收到调用，中心应位于距离顶点 10 的真实裁切平面。
func TestMiterClipGapCallback(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	r := rasterx.NewStroker(128, 128, rasterx.NewScannerGV(128, 128, img, img.Bounds()))
	calls := 0
	gap := func(p rasterx.Adder, center, a, b fixed.Point26_6) {
		calls++
		x, y := 64+30/math.Sqrt(10), 64-10/math.Sqrt(10)
		if math.Hypot(float64(center.X)/64-x, float64(center.Y)/64-y) > 1.0/64 {
			t.Errorf("gap 中心=%v，期望 (%.5f,%.5f)", center, x, y)
		}
		rasterx.FlatGap(p, center, a, b)
	}
	r.SetStroke(10*64, 2*64, nil, nil, gap, rasterx.MiterClip)
	r.Joiner(rasterx.C2Point{P: rasterx.ToFixedP(64, 64), TTan: rasterx.ToFixedP(4, 0), LTan: rasterx.ToFixedP(-4, 3), TNorm: rasterx.ToFixedP(0, -5), LNorm: rasterx.ToFixedP(3, 4)})
	if calls != 1 {
		t.Fatalf("gap 调用次数=%d，期望 1", calls)
	}
}
