package mascot

import "math"

// This file is a literal port of art/mascot-rig.mjs geometry, materials and
// ray sampling. Operation order, strict `<` nearest-hit comparisons, slab tie
// order and the 1e-10 parallel threshold are preserved deliberately so the
// native renderer reproduces the frozen JS cells.

// vec3 is a private 3-component float64 vector with only the arithmetic the
// renderer needs. It is a value type: no heap allocation per ray.
type vec3 struct{ x, y, z float64 }

func (a vec3) add(b vec3) vec3 { return vec3{a.x + b.x, a.y + b.y, a.z + b.z} }
func (a vec3) sub(b vec3) vec3 { return vec3{a.x - b.x, a.y - b.y, a.z - b.z} }

func (a vec3) scale(s float64) vec3 { return vec3{a.x * s, a.y * s, a.z * s} }

func (a vec3) dot(b vec3) float64 { return a.x*b.x + a.y*b.y + a.z*b.z }

func (a vec3) unit() vec3 {
	return a.scale(1 / math.Sqrt(a.dot(a)))
}

func (a vec3) axis(i int) float64 {
	switch i {
	case 0:
		return a.x
	case 1:
		return a.y
	default:
		return a.z
	}
}

func (a *vec3) setAxis(i int, v float64) {
	switch i {
	case 0:
		a.x = v
	case 1:
		a.y = v
	default:
		a.z = v
	}
}

// role is the semantic material role a sample keeps for encoding.
type role uint8

const (
	roleBackground role = iota
	roleFur
	roleEye
	rolePupil
	roleNose
	roleMouth
	roleTooth
)

// partID identifies a part for the mouth rule and diagnostics.
type partID uint8

const (
	partBody partID = iota
	partHead
	partEarLeft
	partEarRight
	partEyeLeft
	partEyeRight
	partMuzzle
	partNose
	partToothLeft
	partToothRight
)

// part is a fixed ellipsoid, or a box when box is set. fixed means the part
// stays in world space and never rotates or bobs.
type part struct {
	center vec3
	radii  vec3
	role   role
	id     partID
	box    bool
	fixed  bool
}

// rigParts order is body, head, ears, eyes, muzzle, nose, then teeth. Keep it
// exactly as in the JS rig; nearest-hit ties resolve to the earlier part.
var rigParts = [10]part{
	{center: vec3{0, -0.64, -0.18}, radii: vec3{0.78, 0.36, 0.50}, role: roleFur, id: partBody, fixed: true},
	{center: vec3{0, 0, 0}, radii: vec3{0.94, 0.95, 0.68}, role: roleFur, id: partHead},
	{center: vec3{-0.92, 0.50, -0.08}, radii: vec3{0.20, 0.21, 0.16}, role: roleFur, id: partEarLeft},
	{center: vec3{0.92, 0.50, -0.08}, radii: vec3{0.20, 0.21, 0.16}, role: roleFur, id: partEarRight},
	{center: vec3{-0.37, 0.34, 0.60}, radii: vec3{0.34, 0.37, 0.17}, role: roleEye, id: partEyeLeft},
	{center: vec3{0.37, 0.34, 0.60}, radii: vec3{0.34, 0.37, 0.17}, role: roleEye, id: partEyeRight},
	{center: vec3{0, -0.18, 0.72}, radii: vec3{0.30, 0.19, 0.10}, role: roleFur, id: partMuzzle},
	{center: vec3{0, 0.04, 0.83}, radii: vec3{0.063, 0.035, 0.035}, role: roleNose, id: partNose},
	{center: vec3{-0.09, -0.34, 0.84}, radii: vec3{0.06, 0.080, 0.025}, role: roleTooth, id: partToothLeft, box: true},
	{center: vec3{0.09, -0.34, 0.84}, radii: vec3{0.06, 0.080, 0.025}, role: roleTooth, id: partToothRight, box: true},
}

var (
	rigPivot = vec3{0, 0.20, 0}
	rigLight = vec3{-0.5, 0.8, 1}.unit()
)

// rotation stores sin/cos once per pose and provides the two small transform
// helpers. Forward is R = Ry(yaw) * Rx(pitch); inverse is its transpose.
type rotation struct {
	cy, sy, cp, sp float64
}

func newRotation(yawDeg, pitchDeg float64) rotation {
	yaw := yawDeg * math.Pi / 180
	pitch := pitchDeg * math.Pi / 180
	return rotation{
		cy: math.Cos(yaw), sy: math.Sin(yaw),
		cp: math.Cos(pitch), sp: math.Sin(pitch),
	}
}

func (r rotation) forward(v vec3) vec3 {
	yy := r.cp*v.y - r.sp*v.z
	zz := r.sp*v.y + r.cp*v.z
	return vec3{r.cy*v.x + r.sy*zz, yy, -r.sy*v.x + r.cy*zz}
}

func (r rotation) inverse(v vec3) vec3 {
	xx := r.cy*v.x - r.sy*v.z
	zz := r.sy*v.x + r.cy*v.z
	return vec3{xx, r.cp*v.y + r.sp*zz, -r.sp*v.y + r.cp*zz}
}

// rayHit carries the winning intersection in the space the ray was traced in
// (world for fixed parts, head-local otherwise).
type rayHit struct {
	t      float64
	normal vec3
	point  vec3
}

func clamp(x, lo, hi float64) float64 {
	return math.Min(hi, math.Max(lo, x))
}

// jsSign mirrors JavaScript Math.sign, including preserving a signed zero.
func jsSign(x float64) float64 {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	default:
		return x
	}
}

// intersectEllipsoid is the quadratic ellipsoid test. Transformed directions
// are intentionally not normalised so t stays comparable across parts.
func intersectEllipsoid(origin, direction vec3, p part) (rayHit, bool) {
	o := origin.sub(p.center)
	r := p.radii
	oo := vec3{o.x / r.x, o.y / r.y, o.z / r.z}
	dd := vec3{direction.x / r.x, direction.y / r.y, direction.z / r.z}

	a := dd.dot(dd)
	b := 2 * oo.dot(dd)
	c := oo.dot(oo) - 1
	disc := b*b - 4*a*c
	if disc < 0 {
		return rayHit{}, false
	}
	root := math.Sqrt(disc)
	t := (-b - root) / (2 * a)
	if t <= 0 {
		t = (-b + root) / (2 * a)
	}
	if t <= 0 {
		return rayHit{}, false
	}
	off := o.add(direction.scale(t))
	normal := vec3{
		off.x / (r.x * r.x),
		off.y / (r.y * r.y),
		off.z / (r.z * r.z),
	}.unit()
	return rayHit{t: t, normal: normal, point: origin.add(direction.scale(t))}, true
}

// intersectBox is the ordinary slab test. Parallel slabs use the 1e-10
// threshold; entry/exit axis ties keep the first axis that moves the bound.
func intersectBox(origin, direction vec3, p part) (rayHit, bool) {
	o := origin.sub(p.center)
	r := p.radii
	entry := math.Inf(-1)
	exit := math.Inf(1)
	entryAxis, exitAxis := 0, 0
	for i := 0; i < 3; i++ {
		di := direction.axis(i)
		if math.Abs(di) < 1e-10 {
			if math.Abs(o.axis(i)) > r.axis(i) {
				return rayHit{}, false
			}
			continue
		}
		lo := (-r.axis(i) - o.axis(i)) / di
		hi := (r.axis(i) - o.axis(i)) / di
		if lo > hi {
			lo, hi = hi, lo
		}
		if lo > entry {
			entry = lo
			entryAxis = i
		}
		if hi < exit {
			exit = hi
			exitAxis = i
		}
	}
	if exit < math.Max(entry, 0) {
		return rayHit{}, false
	}
	t := exit
	axis := exitAxis
	if entry > 0 {
		t = entry
		axis = entryAxis
	}
	var normal vec3
	normal.setAxis(axis, jsSign(o.axis(axis)+t*direction.axis(axis)))
	return rayHit{t: t, normal: normal, point: origin.add(direction.scale(t))}, true
}

// sample is one traced ray result: the palette index to paint, the semantic
// role, the underlying fur shade and the head-local X the smile rule needs.
type sample struct {
	color uint8
	role  role
	under uint8
	part  partID
	x     float64
}

// material applies the shaded fur index plus the eye/pupil/lid, nose, tooth
// and mouth material regions in head-local coordinates.
func material(h rayHit, p part, pose Pose, rot rotation) sample {
	normal := h.normal
	if !p.fixed {
		normal = rot.forward(h.normal)
	}
	shade := uint8(clamp(1+math.Floor((0.25+0.75*math.Max(0, normal.dot(rigLight)))*5), 1, 5))
	color := shade
	rl := p.role

	switch rl {
	case roleEye:
		u := (h.point.x - p.center.x) / p.radii.x
		v := (h.point.y - p.center.y) / p.radii.y
		if v > -1+2*pose.EyeOpen {
			// A closed/covered lid is shaded fur, covering white and pupil.
			rl = roleFur
		} else {
			pupilU := -jsSign(p.center.x) * 0.06
			color = 6
			du := (u - pupilU) / 0.18
			dv := (v + 0.10) / 0.18
			if du*du+dv*dv <= 1 {
				color = 7
				rl = rolePupil
			}
		}
	case roleNose:
		color = 7
	case roleTooth:
		color = 6
	}

	if p.id == partHead || p.id == partMuzzle {
		x := h.point.x
		t := clamp((math.Abs(x)-0.15)/0.09, 0, 1)
		y := -0.195 + pose.Lift*t*t
		if math.Abs(x) <= 0.24 && math.Abs(h.point.y-y) <= 0.030 && h.normal.z > 0 {
			rl = roleMouth
			color = 7
		}
	}

	return sample{color: color, role: rl, under: shade, part: p.id, x: h.point.x}
}

// sampleScene traces the fixed 64x48 grid for one sanitized pose. Body rays
// stay in world space; head rays are inverse-transformed to head-local space.
func sampleScene(pose Pose, samples *[SampleWidth * SampleHeight]sample) {
	rot := newRotation(pose.Yaw, pose.Pitch)
	direction := vec3{0, 0, -1}
	localDirection := rot.inverse(direction)
	pivot := rigPivot.add(vec3{0, pose.Bob, 0})

	for sy := 0; sy < SampleHeight; sy++ {
		for sx := 0; sx < SampleWidth; sx++ {
			origin := vec3{
				x: (float64(sx) + 0.5 - 32) / 20,
				y: 0.15 + (24-float64(sy)-0.5)/20,
				z: 3,
			}
			localOrigin := rot.inverse(origin.sub(pivot))

			found := false
			var nearest rayHit
			nearestPart := 0
			for i := range rigParts {
				p := &rigParts[i]
				o := origin
				d := direction
				if !p.fixed {
					o = localOrigin
					d = localDirection
				}
				var h rayHit
				var ok bool
				if p.box {
					h, ok = intersectBox(o, d, *p)
				} else {
					h, ok = intersectEllipsoid(o, d, *p)
				}
				if ok && (!found || h.t < nearest.t) {
					nearest = h
					nearestPart = i
					found = true
				}
			}

			idx := sy*SampleWidth + sx
			if !found {
				samples[idx] = sample{color: 0, role: roleBackground}
				continue
			}
			samples[idx] = material(nearest, rigParts[nearestPart], pose, rot)
		}
	}
}
