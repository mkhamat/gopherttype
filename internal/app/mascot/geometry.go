package mascot

import "math"

// This file is a literal port of art/mascot-rig.mjs geometry, materials and
// ray sampling. Operation order, strict `<` nearest-hit comparisons, rounded
// plate corner tests and the 1e-10 parallel threshold are preserved
// deliberately so the native renderer reproduces the frozen JS cells.

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
	roleEar
	roleMuzzle
	roleSeam
)

// partID identifies a part for diagnostics.
type partID uint8

const (
	partBody partID = iota
	partHead
	partCheeks
	partEarLeft
	partEarRight
	partEarInnerLeft
	partEarInnerRight
	partEyeLeft
	partEyeRight
	partPupilLeft
	partPupilRight
	partMuzzleLeft
	partMuzzleRight
	partMouth
	partNose
	partToothDivider
	partToothLeft
	partToothRight
)

// part is an ellipsoid, or a rounded plate when corner is nonzero. fixed means
// the part stays in world space and never rotates or bobs.
type part struct {
	center vec3
	radii  vec3
	role   role
	id     partID
	corner float64 // >0 marks a rounded plate; 0 is an ellipsoid
	fixed  bool
	parent int // parent eye index for the pupil lid; read only when role is pupil
}

// rigParts order is exactly the JS rig: body, head, cheeks, ears, inner ears,
// eyes, pupils, muzzle halves, mouth and nose ellipsoids, then the rounded
// teeth plates (divider, left, right). Ties resolve to the earlier part.
var rigParts = [18]part{
	{center: vec3{0, -0.78, -0.16}, radii: vec3{0.86, 0.42, 0.48}, role: roleFur, id: partBody, fixed: true},
	{center: vec3{0, 0.10, 0}, radii: vec3{1.00, 0.90, 0.64}, role: roleFur, id: partHead},
	{center: vec3{0, -0.50, -0.04}, radii: vec3{0.93, 0.70, 0.62}, role: roleFur, id: partCheeks},
	{center: vec3{-0.91, 0.53, -0.08}, radii: vec3{0.225, 0.23, 0.15}, role: roleFur, id: partEarLeft},
	{center: vec3{0.91, 0.53, -0.08}, radii: vec3{0.225, 0.23, 0.15}, role: roleFur, id: partEarRight},
	{center: vec3{-0.945, 0.54, 0.065}, radii: vec3{0.085, 0.095, 0.035}, role: roleEar, id: partEarInnerLeft},
	{center: vec3{0.945, 0.54, 0.065}, radii: vec3{0.085, 0.095, 0.035}, role: roleEar, id: partEarInnerRight},
	{center: vec3{-0.385, 0.45, 0.63}, radii: vec3{0.32, 0.325, 0.13}, role: roleEye, id: partEyeLeft},
	{center: vec3{0.385, 0.45, 0.63}, radii: vec3{0.32, 0.325, 0.13}, role: roleEye, id: partEyeRight},
	{center: vec3{-0.435, 0.425, 0.78}, radii: vec3{0.085, 0.095, 0.025}, role: rolePupil, id: partPupilLeft, parent: 7},
	{center: vec3{0.335, 0.425, 0.78}, radii: vec3{0.085, 0.095, 0.025}, role: rolePupil, id: partPupilRight, parent: 8},
	{center: vec3{-0.095, -0.02, 0.72}, radii: vec3{0.17, 0.115, 0.09}, role: roleMuzzle, id: partMuzzleLeft},
	{center: vec3{0.095, -0.02, 0.72}, radii: vec3{0.17, 0.115, 0.09}, role: roleMuzzle, id: partMuzzleRight},
	{center: vec3{0, -0.145, 0.735}, radii: vec3{0.185, 0.055, 0.035}, role: roleMouth, id: partMouth},
	{center: vec3{0, 0.16, 0.79}, radii: vec3{0.14, 0.075, 0.07}, role: roleNose, id: partNose},
	{center: vec3{0, -0.235, 0.745}, radii: vec3{0.026, 0.12, 0.03}, role: roleSeam, id: partToothDivider, corner: 0.008},
	{center: vec3{-0.10, -0.235, 0.745}, radii: vec3{0.075, 0.105, 0.045}, role: roleTooth, id: partToothLeft, corner: 0.035},
	{center: vec3{0.10, -0.235, 0.745}, radii: vec3{0.075, 0.105, 0.045}, role: roleTooth, id: partToothRight, corner: 0.035},
}

// neutralLift is the rig's neutral smile-corner lift. Muzzle and mouth parts
// are offset by (lift - neutralLift) * 0.6 along head-local Y.
const neutralLift = 0.06

var (
	rigPivot = vec3{0, 0.14, 0}
	rigLight = vec3{-0.55, 0.75, 1}.unit()
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

// intersect dispatches the ellipsoid and rounded-plate tests exactly as the JS
// rig branches on part.corner.
func intersect(origin, direction vec3, p part) (rayHit, bool) {
	if p.corner != 0 {
		return intersectPlate(origin, direction, p)
	}
	return intersectEllipsoid(origin, direction, p)
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

// intersectPlate is the rounded-rectangle plate test. The flat face sits at
// +z; a ray parallel to z (within 1e-10) misses. The corner rounding uses the
// exact JS rounded-box signed distance.
func intersectPlate(origin, direction vec3, p part) (rayHit, bool) {
	if math.Abs(direction.z) < 1e-10 {
		return rayHit{}, false
	}
	o := origin.sub(p.center)
	r := p.radii
	t := (r.z - o.z) / direction.z
	if t <= 0 {
		return rayHit{}, false
	}
	point := origin.add(direction.scale(t))
	x := math.Abs(point.x-p.center.x) - r.x + p.corner
	y := math.Abs(point.y-p.center.y) - r.y + p.corner
	if math.Hypot(math.Max(x, 0), math.Max(y, 0))+math.Min(math.Max(x, y), 0) > p.corner {
		return rayHit{}, false
	}
	return rayHit{t: t, normal: vec3{0, 0, 1}, point: point}, true
}

// sample is one traced ray result: the palette index to paint and the semantic
// role that weights pair selection.
type sample struct {
	color uint8
	role  role
	part  partID
}

// material applies the smooth shading envelope, flat lighting and the
// role-specific palette regions in the space the ray was traced in.
func material(h rayHit, p part, pose Pose, rot rotation) sample {
	point := h.point
	localNormal := h.normal
	if p.role == roleEye || p.role == rolePupil || p.id == partBody || p.id == partHead || p.id == partCheeks {
		pivotY := 0.0
		if p.fixed {
			pivotY = rigPivot.y
		}
		localNormal = vec3{point.x, (point.y + 0.24 - pivotY) / 1.56, point.z / 0.41}.unit()
	}
	normal := localNormal
	if !p.fixed {
		normal = rot.forward(localNormal)
	}
	shade := uint8(clamp(1+math.Floor((0.18+0.82*math.Max(0, normal.dot(rigLight)))*4.99), 1, 5))
	color := shade
	rl := p.role

	switch rl {
	case roleEye, rolePupil:
		eye := p
		if rl == rolePupil {
			eye = rigParts[p.parent]
		}
		lid := eye.center.y + eye.radii.y*(-1+2*pose.EyeOpen)
		if point.y > lid {
			// The lid covers eye white and pupil together with shaded fur.
			rl = roleFur
		} else if p.role == rolePupil {
			color = 7
		} else {
			color = 6
		}
	case roleNose, roleEar, roleMouth, roleSeam:
		color = 7
	case roleMuzzle:
		color = 8
	case roleTooth:
		color = 6
	}

	return sample{color: color, role: rl, part: p.id}
}

// sampleScene traces the fixed 128x96 grid for one sanitized pose. Body rays
// stay in world space; head rays are inverse-transformed to head-local space.
// Muzzle and mouth parts follow the smile-corner lift.
func sampleScene(pose Pose, samples *[SampleWidth * SampleHeight]sample) {
	rot := newRotation(pose.Yaw, pose.Pitch)
	direction := vec3{0, 0, -1}
	localDirection := rot.inverse(direction)
	pivot := rigPivot.add(vec3{0, pose.Bob, 0})
	lift := (pose.Lift - neutralLift) * 0.6

	var parts [18]part
	for i := range rigParts {
		parts[i] = rigParts[i]
		if parts[i].role == roleMuzzle || parts[i].role == roleMouth {
			parts[i].center.y += lift
		}
	}

	for sy := 0; sy < SampleHeight; sy++ {
		for sx := 0; sx < SampleWidth; sx++ {
			origin := vec3{
				x: -0.025 + (float64(sx)+0.5-float64(SampleWidth)/2)/40,
				y: 0.15 + (float64(SampleHeight)/2-float64(sy)-0.5)/40,
				z: 3,
			}
			localOrigin := rot.inverse(origin.sub(pivot))

			found := false
			var nearest rayHit
			nearestPart := 0
			for i := range parts {
				p := &parts[i]
				o, d := origin, direction
				if !p.fixed {
					o, d = localOrigin, localDirection
				}
				h, ok := intersect(o, d, *p)
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
			samples[idx] = material(nearest, parts[nearestPart], pose, rot)
		}
	}
}
