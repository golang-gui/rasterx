# 路径精度与 SVG 描边正确性修正

## 范围

修复发生在 rasterx 的描边几何层；不在 oksvg 中删除路径、添加图标特例或替换图形。
保持纯 Go、Scanner 接口和既有调用方式。SVG 解析、文档样式和扫描器的像素合成不进入本层。

## 圆角连接的反向折返

原 `strokeEdge` 用叉积接近零判断“无需补齐连接”。同向共线和反向共线的叉积均为零，
但 `Round` 连接在反向折返处需要半圆，否则丢失真实轮廓。

保留原同向与非退化连接路径；在叉积为零且法向量点积为负的 `Round` 连接处调用
既有 `JoinGap`，使原有圆角及自定义 gap 契约继续生效。没有将 bevel/miter 改为圆角。

最小复现：线宽 12，`(40,64) -> (80,64) -> (40,64)`，round cap/join。
像素 `(84,64)` 的中心距折返点约 4.53，小于半径 6，应处于不透明轮廓内部；旧实现为透明。

## 曲线内部的折返

共线 Bézier 曲线可能在一个命令内部出现驻点并原路返回。均匀采样可能跨过驻点，
产生反向或零切线；只修复命令之间的 Joiner 不足以恢复这种轮廓。

- 先检查控制点是否共线，普通曲线沿用原实现。
- 二次曲线的一阶导数是一次多项式，三次曲线的一阶导数是二次多项式，分别求根。
- 仅取 `(0,1)` 内发生方向改变的根；三次曲线的重根没有方向反转，无需拆分。
- 用 De Casteljau 求各驻点并按 fixed-point 精度取整，将各段作为精确直线处理。
- 原命令起点与前一命令的连接保留调用方配置；曲线内部折返使用圆形笔刷扫过的轮廓。
  处理结束恢复 `JoinMode` 和 `JoinGap`，不污染后续命令。
- 复用已有线段及 Dasher 路径，使共线曲线的距离仍用于虚线推进；不按参数 t 均匀分配虚线长度。

## 验证标准

`stroke_test.go` 包含：

- 水平、垂直、斜向和近似折返，同向连续和直角；1x、2x、4x。
- 通过独立的点到线段距离检查圆角折线，而非比较两个渲染器是否同错。
- 距理论边界 1.25 物理像素之外检查精确内部颜色与外部透明；这是边缘覆盖率的留白，
  不能用于放宽内部采样的颜色断言。
- 二次、三次及含两个驻点的共线曲线；分别使用 Round、Bevel、Miter，验证内部折返不受命令连接样式影响。

二次 `40 -> 120 -> 40` 的最远点为 80；三次 `40 -> 100 -> 100 -> 40` 的最远点为 85。
测试同时断言最远点之外 4 像素仍在描边内、8 像素处透明，防止修复造成轮廓扩大。

真实图标经 oksvg + scanFT 在 24、64、128px 验证，直接读取固定版本 Lucide 原始输入。
包括 braces、dumbbell、squirrel、mop 的对照与逐项结果记录在 oksvg 的 `doc/CorpusValidation.md`。
Chrome 与合成几何用例交叉核对；不把单个参考实现作为所有 SVG 版本的无条件真值。

## 开发与验证状态

- [x] 反向圆角连接修正及独立解析几何回归。
- [x] 共线曲线内部折返及双驻点回归。
- [x] 三库全量测试；rasterx 与 scanFT race 检查。
- [x] `CGO_ENABLED=0` Windows amd64、Linux arm64、macOS arm64 测试目标编译。
- [x] 独立提交模块路径迁移与各项修复。
- [ ] 从 master 移植修复到上游 PR 分支。
- [ ] 发布正式 fork 版本并升级消费方依赖；GOUI 接入另行授权。

当前 `main` 的模块路径为 `github.com/golang-gui/rasterx`。联调使用仓库外的临时
`go.work` 和本地源码副本，旧测试生成的 PNG 不写入源码仓库。仅执行了 Linux amd64
原生测试；其他目标为编译检查，不代表已完成原生运行验收。

## 依据

- [SVG 2 stroke shape](https://www.w3.org/TR/SVG2/painting.html#StrokeShape)
- [SVG 2 path implementation notes](https://www.w3.org/TR/SVG2/paths.html#PathElementImplementationNotes)

## 后续阶段：边缘、虚线与 miter-clip（2026-10-29）

本阶段基线为 main 的 `786a0e1`，在原有纯 Go API 内修正数值与几何契约。

### 坐标量化与重复采样

`ToFixedP`、`Matrix2D.TFixed` 和曲线离散顶点统一向最近的 1/64 坐标单位取整，
半步按 Go `math.Round` 规则远离零。正负坐标对称，避免浮点尾差将整数边界向内缩一格。
例如略小于 16 的浮点数应量化为 16，而非 15+63/64。

曲线的重复定点采样不携带方向，`Stroker.lineF` 跳过这些点并保留上一法向量。
原先重复点会得到零法向量，将描边收缩到中心线，导致圆端帽内部出现半透明缺口。
显式零长度线段的端帽仍由既有命令连接和 Stop 逻辑处理。

覆盖：`TestFixedPointRounding`、`TestBezierSamplesOnStraightAxis`、`TestBezierEndpointCoverage`。
所有控制点位于 y=64 的曲线必须保持 y=64；圆端帽内部使用独立圆盘距离断言不透明。

### 虚线距离与端帽

- 保留公开 Dashes / DashOffset 的定点类型，内部累计距离使用 float64。
- 小段长度以完整欧氏长度累计，仅在输出轮廓顶点时量化，消除逐段截断的相位漂移。
- 奇数模式重复一次；正负及大 offset 按完整周期归一化，再量化为定点。
- dash 恰好在命令顶点结束时先结束该 dash，不生成跨 dash 的连接；零长度间隔仍区分两个 dash。
- gap 在顶点结束时，将下一个 dash 的起始端帽推迟到取得出方向切线后绘制。
  路径在这里结束则没有下一条 dash，不能绘制多余端帽。
- 绘制期间出现的零长度离散段不推进距离。全零模式继续采用既有实线契约。

覆盖：`TestDashDistance` 比较一段与 448 个共线小段，并使用解析相位判断像素；
`TestOddDashPattern` 验证奇数模式；`TestDashBoundaryAtCorner` 验证零间隔不产生 miter。
`TestDashCapsAtVertices` 独立列出 L 形路径上的 dash 区间，用圆盘/矩形距离验证
round、square、butt 与普通间隔、零间隔、零长度 dash 的九种组合。

`FuzzDashSegments` 对有界路径、重复点、负偏移、零间隔执行无 panic 与预乘像素检查。
该测试文件使用 go1.18 构建约束；生产库的 go.mod 仍为 Go 1.17。

### miter-clip 裁切平面

旧算法把投影比例过早量化到 1/64，并使用不准确的中心重建 gap。
修正后用浮点求两条外侧边与裁切平面的交点，最终顶点才转定点；平面垂直于 miter 轴，
距连接点为半线宽乘 miterLimit。保留 JoinGap 回调，传入位于真实平面上的中心与端点。
本分支也用于零曲率的 ArcClip；非零曲率 ArcClip 的独立算法不在本轮修改范围。

`TestMiterClipPlane` 使用精确的 3-4-5 法向量，独立求交点，允许误差为一个定点单位；
`TestMiterClipGapCallback` 验证回调保留且中心正确。

Chrome 156.0.8078.47 实测不支持 miter-clip，计算样式回退为 miter，不能作为该属性的验收参考。
本项使用数学构造及 resvg 固定参考 PNG：500px 输出的 stable_bad/geometry_bad 均为零，
原有边缘差异进一步减小。虚线 n-0 的原始参考也支持分离端帽；旧版本 librsvg 的结果不同。

依据：

- [SVG 2 dash positions](https://www.w3.org/TR/SVG2/painting.html#StrokeShape)
- [SVG Strokes miter-clip](https://w3c.github.io/svgwg/specs/strokes/#StrokeLinejoinProperty)
- [早期裁切描述的规范讨论](https://github.com/w3c/svgwg/issues/745)

### 验证与后续

- [x] rasterx / scanFT / oksvg 全量测试和 race。
- [x] Dasher fuzz 15 秒，132,349 次；解析 fuzz 15 秒，564,963 次；无失败。
- [x] 三库共九项 CGO_ENABLED=0 目标编译：Windows amd64、Linux arm64、macOS arm64。
- [x] 5,756 份原始 SVG 的三个尺寸对照，及额外合成/样式回归；详见 oksvg CorpusValidation。
- [x] 本轮代码按坐标精度、端帽、虚线、miter-clip 独立提交。
- [ ] 从 master 移植修复到上游 PR 分支；GOUI 依赖升级另按指令执行。

原生运行环境为 Linux amd64、Go 1.25.8；跨目标仅编译，不代表其他平台原生运行通过。
本阶段不改变扫描器选择，不增加高精度路径模型，也不声称消除所有亚像素差异。

## 后续阶段：近直线曲线的端部

resvg `line-as-curve-2` 的控制点紧邻端点，短控制边的方向与主轴相差约 45°。
旧实现保持该短切线的端帽，却使用跨越该区域的均匀离散段连接，形成可见的端部尖角。
这不是普通边缘覆盖率差异。

采用有界的近直线简化：二次/三次控制点到端点弦的距离均不超过既有
`epsilonFixed=16`（1/4 光栅坐标单位），并且控制点沿弦的投影单调位于两端之间，
才以现有 LineSeg 路径生成轮廓及端帽。Bezier 的凸包性质给出中心线的距离上界；
该规则是退化曲线的近似策略，不声称短端部的数学切线与弦完全相同。
不满足条件的曲线继续采用原曲线路径，严格共线折返仍先用驻点拆分处理。
Filler 的路径不简化，不修改输入及 join/cap 状态，也不把这种近似扩展到任意曲线。

验证分别使用独立直线端帽的内外区域、非直线/折返控制组和原始 SVG 套件。
检查 butt / round / square、旋转、缩放和虚线；超出误差限的控制组必须继续保留曲率。

参考 Skia `SkStroke.cpp` 的 CheckCubicLinear / CheckQuadLinear：先分类退化、近直线曲线，
再进入完整描边。本实现使用光栅坐标的绝对误差上限，不照搬随路径长度增大的相对容差。

- [Skia 描边实现](https://github.com/google/skia/blob/main/src/core/SkStroke.cpp)
- [x] 近直线简化与独立回归：butt / round / square、1x/2x/4x、旋转和虚线。
- [x] 控制组排除越界、非单调、明显弯曲、首尾重合及放大后超限的曲线。
- [x] 原始 SVG 对照及三库全量、race、fuzz、目标编译检查。

resvg line-as-curve-2 在 24/64/128px 与 Chrome 对照、500px 与套件 PNG 对照，
stable_bad / geometry_bad 均为 0。全量 Lucide / Bootstrap 没有新增筛选候选，
普通边缘差异仍保留；跨库结果见 oksvg 的 CorpusValidation 第四阶段。
