package rasterx_test

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"testing"

	"github.com/golang-gui/rasterx"
)

// 直线长度有解析解，分成许多短段后仍须保持同一虚线相位。
func TestDashDistance(t *testing.T) {
	for _, steps := range []int{1, 448} {
		for _, offset := range []float64{0, 5, -5, 37, 1600000005, -1600000005} {
			t.Run(strconv.Itoa(steps)+"/"+strconv.Itoa(int(offset)), func(t *testing.T) {
				img := image.NewRGBA(image.Rect(0, 0, 144, 144))
				r := rasterx.NewDasher(144, 144, rasterx.NewScannerGV(144, 144, img, img.Bounds()))
				r.SetStroke(4*64, 4*64, rasterx.ButtCap, nil, nil, rasterx.Miter, []float64{8, 8}, offset)
				r.SetColor(color.RGBA{A: 255})
				r.Start(rasterx.ToFixedP(8, 8))
				for i := 1; i <= steps; i++ {
					p := 8 + 112*float64(i)/float64(steps)
					r.Line(rasterx.ToFixedP(p, p))
				}
				r.Stop(false)
				r.Draw()
				for x := 10; x < 118; x++ {
					distance := math.Sqrt2 * (float64(x) + .5 - 8)
					phase := math.Mod(distance+offset, 16)
					if phase < 0 {
						phase += 16
					}
					if math.Min(math.Mod(phase, 8), 8-math.Mod(phase, 8)) < 1.25 {
						continue
					}
					want := uint8(0)
					if phase < 8 {
						want = 255
					}
					if got := img.RGBAAt(x, x).A; got != want {
						t.Fatalf("(%d,%d) alpha=%d，期望 %d；距离 %.3f，相位 %.3f", x, x, got, want, distance, phase)
					}
				}
			})
		}
	}
}

// 单元素的奇数模式按 SVG 规则重复一次，负 offset 也按完整周期计算。
func TestOddDashPattern(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 96, 32))
	r := rasterx.NewDasher(96, 32, rasterx.NewScannerGV(96, 32, img, img.Bounds()))
	r.SetStroke(4*64, 4*64, rasterx.ButtCap, nil, nil, rasterx.Miter, []float64{8}, -5)
	r.SetColor(color.RGBA{A: 255})
	r.Start(rasterx.ToFixedP(8, 16))
	r.Line(rasterx.ToFixedP(88, 16))
	r.Stop(false)
	r.Draw()
	for x := 9; x < 87; x++ {
		phase := math.Mod(float64(x)+.5-8-5, 16)
		if phase < 0 {
			phase += 16
		}
		want := uint8(0)
		if phase < 8 {
			want = 255
		}
		if got := img.RGBAAt(x, 16).A; got != want {
			t.Fatalf("x=%d alpha=%d，期望 %d", x, got, want)
		}
	}
}

// 虚线在顶点处结束，即使下一个间隔长度为零，也不应生成跨越两个 dash 的 miter。
func TestDashBoundaryAtCorner(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	r := rasterx.NewDasher(128, 128, rasterx.NewScannerGV(128, 128, img, img.Bounds()))
	r.SetStroke(8*64, 4*64, rasterx.ButtCap, nil, nil, rasterx.Miter, []float64{8, 0}, 0)
	r.SetColor(color.RGBA{A: 255})
	r.Start(rasterx.ToFixedP(20, 20))
	for _, p := range [][2]float64{{100, 20}, {100, 100}, {20, 100}} {
		r.Line(rasterx.ToFixedP(p[0], p[1]))
	}
	r.Stop(true)
	r.Draw()
	for _, p := range [][2]int{{18, 18}, {102, 18}, {102, 102}, {18, 102}} {
		if got := img.RGBAAt(p[0], p[1]).A; got != 0 {
			t.Errorf("角外 (%d,%d) alpha=%d，期望透明", p[0], p[1], got)
		}
	}
	for _, p := range [][2]int{{60, 20}, {100, 60}, {60, 100}, {20, 60}} {
		if got := img.RGBAAt(p[0], p[1]).A; got != 255 {
			t.Errorf("边上 (%d,%d) alpha=%d，期望不透明", p[0], p[1], got)
		}
	}
}

// 每个 dash 的区间由长度 80 的 L 形路径独立列出；端帽使用圆盘或矩形的解析距离。
func TestDashCapsAtVertices(t *testing.T) {
	for _, pattern := range []struct {
		name      string
		values    []float64
		intervals [][2]float64
	}{
		{"regular", []float64{10, 10}, [][2]float64{{0, 10}, {20, 30}, {40, 50}, {60, 70}}},
		{"zero gap", []float64{10, 0}, [][2]float64{{0, 10}, {10, 20}, {20, 30}, {30, 40}, {40, 50}, {50, 60}, {60, 70}, {70, 80}}},
		{"dots", []float64{0, 10}, [][2]float64{{0, 0}, {10, 10}, {20, 20}, {30, 30}, {40, 40}, {50, 50}, {60, 60}, {70, 70}}},
	} {
		for _, cap := range []struct {
			name string
			fn   rasterx.CapFunc
		}{{"round", rasterx.RoundCap}, {"square", rasterx.SquareCap}, {"butt", rasterx.ButtCap}} {
			t.Run(pattern.name+"/"+cap.name, func(t *testing.T) {
				img := image.NewRGBA(image.Rect(0, 0, 80, 80))
				r := rasterx.NewDasher(80, 80, rasterx.NewScannerGV(80, 80, img, img.Bounds()))
				r.SetStroke(6*64, 4*64, cap.fn, nil, nil, rasterx.Miter, pattern.values, 0)
				r.SetColor(color.RGBA{B: 255, A: 255})
				r.Start(rasterx.ToFixedP(20, 20))
				r.Line(rasterx.ToFixedP(60, 20))
				r.Line(rasterx.ToFixedP(60, 60))
				r.Stop(false)
				r.Draw()
				for y := 0; y < 80; y++ {
					for x := 0; x < 80; x++ {
						px, py := float64(x)+.5, float64(y)+.5
						distance := math.Inf(1)
						for _, interval := range pattern.intervals {
							start, end := interval[0], interval[1]
							if start == end && cap.name == "butt" {
								continue
							}
							var along, across float64
							if start < 40 {
								along, across = px-20, py-20
							} else {
								along, across = py-20+40, px-60
							}
							if cap.name == "round" {
								dx := math.Max(start-along, math.Max(0, along-end))
								distance = math.Min(distance, math.Hypot(dx, across)-3)
							} else {
								extend := 0.0
								if cap.name == "square" {
									extend = 3
								}
								dx := math.Abs(along-(start+end)/2) - (end-start)/2 - extend
								dy := math.Abs(across) - 3
								sd := math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0)
								distance = math.Min(distance, sd)
							}
						}
						if math.Abs(distance) < 1.25 {
							continue
						}
						want := uint8(0)
						if distance < 0 {
							want = 255
						}
						if got := img.RGBAAt(x, y).A; got != want {
							t.Fatalf("(%d,%d) alpha=%d，期望 %d；到解析轮廓的距离 %.3f", x, y, got, want, distance)
						}
					}
				}
			})
		}
	}
}
