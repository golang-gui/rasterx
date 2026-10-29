package rasterx

import (
	"testing"

	"golang.org/x/image/math/fixed"
)

// 独立控制组约束简化条件，防止把闭环、折返或明显弯曲错误地压成直线。
func TestNearLinearCurveBounds(t *testing.T) {
	for _, test := range []struct {
		name   string
		points [][2]float64
		want   bool
	}{
		{"三次近直线", [][2]float64{{20, 100}, {20.0625, 100}, {100, 20}, {100.0625, 20}}, true},
		{"二次近直线", [][2]float64{{20, 100}, {20.0625, 100}, {100, 20}}, true},
		{"负坐标近直线", [][2]float64{{-100, -20}, {-100, -20.0625}, {-20, -100}}, true},
		{"明显弯曲", [][2]float64{{20, 100}, {20, 20}, {100, 20}, {100, 100}}, false},
		{"略超四分之一", [][2]float64{{20, 100}, {20, 100.265625}, {60, 100}, {100, 100}}, false},
		{"同样几何放大后超限", [][2]float64{{200, 1000}, {200.625, 1000}, {1000, 200}, {1000.625, 200}}, false},
		{"控制点越过起点", [][2]float64{{20, 100}, {19, 100}, {100, 100}}, false},
		{"控制点越过终点", [][2]float64{{20, 100}, {101, 100}, {100, 100}}, false},
		{"投影折返", [][2]float64{{20, 100}, {90, 100.125}, {30, 100.125}, {100, 100}}, false},
		{"首尾重合", [][2]float64{{20, 100}, {90, 100.125}, {30, 100.125}, {20, 100}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var points [4]fixed.Point26_6
			for i, p := range test.points {
				points[i] = ToFixedP(p[0], p[1])
			}
			if got := nearLinearCurve(points, len(test.points)); got != test.want {
				t.Fatalf("近直线判定=%v，期望 %v", got, test.want)
			}
		})
	}
}
