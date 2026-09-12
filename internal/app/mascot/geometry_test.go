package mascot

import (
	"math"
	"testing"
)

// vecClose compares two vectors with a small tolerance. Only continuous
// geometry assertions use a tolerance; discrete cell results never do.
func vecClose(a, b vec3) bool {
	const eps = 1e-9
	return math.Abs(a.x-b.x) < eps && math.Abs(a.y-b.y) < eps && math.Abs(a.z-b.z) < eps
}

func TestRotationInverseAndSign(t *testing.T) {
	for _, tc := range []struct{ yaw, pitch float64 }{{0, 0}, {35, 20}, {-35, 0}, {12.34, 7.77}} {
		rot := newRotation(tc.yaw, tc.pitch)
		for _, v := range []vec3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {-0.3, 0.7, 0.2}, {2, -1, 3}} {
			if got := rot.inverse(rot.forward(v)); !vecClose(got, v) {
				t.Errorf("yaw %g pitch %g: inverse(forward(%v)) = %v", tc.yaw, tc.pitch, v, got)
			}
			if got := rot.forward(rot.inverse(v)); !vecClose(got, v) {
				t.Errorf("yaw %g pitch %g: forward(inverse(%v)) = %v", tc.yaw, tc.pitch, v, got)
			}
		}
	}

	// Positive yaw sends +x toward -z (screen-right turn), per the ported sign.
	rot := newRotation(30, 0)
	if got := rot.forward(vec3{1, 0, 0}); got.x <= 0 || got.z >= 0 {
		t.Errorf("positive yaw forward(1,0,0) = %v, want +x/-z", got)
	}
	// Positive pitch rotates +y toward +z.
	rot = newRotation(0, 30)
	if got := rot.forward(vec3{0, 1, 0}); got.y <= 0 || got.z <= 0 {
		t.Errorf("positive pitch forward(0,1,0) = %v, want +y/+z", got)
	}
}

func TestJSSign(t *testing.T) {
	if got := jsSign(5); got != 1 {
		t.Errorf("jsSign(5) = %g", got)
	}
	if got := jsSign(-5); got != -1 {
		t.Errorf("jsSign(-5) = %g", got)
	}
	if got := jsSign(0); got != 0 || math.Signbit(got) {
		t.Errorf("jsSign(0) = %g, want +0", got)
	}
	if got := jsSign(math.Copysign(0, -1)); got != 0 || !math.Signbit(got) {
		t.Errorf("jsSign(-0) = %g, want -0", got)
	}
}

func sphere(center vec3, radius float64) part {
	return part{center: center, radii: vec3{radius, radius, radius}, role: roleFur, id: partHead}
}

// onSurface checks the normalized ellipsoid equation is ~1 at a hit point.
func onSurface(h rayHit, p part) bool {
	d := h.point.sub(p.center)
	u := math.Pow(d.x/p.radii.x, 2) + math.Pow(d.y/p.radii.y, 2) + math.Pow(d.z/p.radii.z, 2)
	return math.Abs(u-1) < 1e-9
}

func TestEllipsoidMissHitTangentInside(t *testing.T) {
	p := sphere(vec3{0, 0, 0}, 1)
	dir := vec3{0, 0, -1}

	// Miss: parallel ray offset beyond the radius.
	if _, ok := intersectEllipsoid(vec3{2, 0, 3}, dir, p); ok {
		t.Error("ellipsoid miss should not hit")
	}
	// Front hit, point on the surface, t positive.
	h, ok := intersectEllipsoid(vec3{0, 0, 3}, dir, p)
	if !ok || h.t <= 0 {
		t.Fatalf("front hit failed: ok=%v t=%g", ok, h.t)
	}
	if !vecClose(h.point, vec3{0, 0, 1}) || !onSurface(h, p) {
		t.Errorf("front hit point %v not the front surface", h.point)
	}
	if !vecClose(h.normal, vec3{0, 0, 1}) {
		t.Errorf("front hit normal %v, want +z", h.normal)
	}
	// Tangent: disc == 0, grazing exactly at x = radius.
	if h, ok := intersectEllipsoid(vec3{1, 0, 3}, dir, p); !ok || !vecClose(h.point, vec3{1, 0, 0}) {
		t.Errorf("tangent hit ok=%v point=%v", ok, h.point)
	}
	// Just past the tangent: miss.
	if _, ok := intersectEllipsoid(vec3{1.000001, 0, 3}, dir, p); ok {
		t.Error("ray just past tangent should miss")
	}
	// Inside: first root is behind the origin, second root is in front.
	h, ok = intersectEllipsoid(vec3{0, 0, 0}, dir, p)
	if !ok || h.t <= 0 || !vecClose(h.point, vec3{0, 0, -1}) {
		t.Errorf("inside hit ok=%v t=%g point=%v", ok, h.t, h.point)
	}
}

func TestEllipsoidUnnormalizedComparableT(t *testing.T) {
	p := sphere(vec3{0, 0, 0}, 1)
	unit, ok1 := intersectEllipsoid(vec3{0, 0, 3}, vec3{0, 0, -1}, p)
	scaled, ok2 := intersectEllipsoid(vec3{0, 0, 3}, vec3{0, 0, -2}, p)
	if !ok1 || !ok2 {
		t.Fatal("both directions should hit")
	}
	if !vecClose(unit.point, scaled.point) {
		t.Errorf("hit point changed with direction scale: %v vs %v", unit.point, scaled.point)
	}
	if math.Abs(unit.t-2*scaled.t) > 1e-9 {
		t.Errorf("t should scale with direction length: unit=%g scaled=%g", unit.t, scaled.t)
	}
}

func TestBoxParallelOutsideInsideAndTie(t *testing.T) {
	box := part{center: vec3{0, 0, 0}, radii: vec3{0.06, 0.08, 0.025}, role: roleTooth, id: partToothLeft, box: true}

	// Parallel to the z axis but outside the slab: miss.
	if _, ok := intersectBox(vec3{0.1, 0, 3}, vec3{0, 0, -1}, box); ok {
		t.Error("parallel ray outside the slab should miss")
	}
	// Parallel and inside the slab: hit the front face.
	if h, ok := intersectBox(vec3{0, 0, 3}, vec3{0, 0, -1}, box); !ok || !vecClose(h.point, vec3{0, 0, 0.025}) {
		t.Errorf("parallel inside hit ok=%v point=%v", ok, h.point)
	}
	// Origin inside: entry is behind, so the exit face is reported.
	h, ok := intersectBox(vec3{0, 0, 0}, vec3{0, 0, -1}, box)
	if !ok || !vecClose(h.point, vec3{0, 0, -0.025}) {
		t.Errorf("inside hit ok=%v point=%v", ok, h.point)
	}
	// Parallel on an axis where the origin sits outside a different slab: miss.
	if _, ok := intersectBox(vec3{0, 0.2, 3}, vec3{0, 0, -1}, box); ok {
		t.Error("ray outside the y slab should miss")
	}

	// Slab-axis tie: a (1,1,1) ray through a unit cube enters/exits on equal
	// bounds; entry/exit ties keep the earliest axis.
	cube := part{center: vec3{0, 0, 0}, radii: vec3{1, 1, 1}, role: roleFur, id: partHead, box: true}
	h, ok = intersectBox(vec3{-2, -2, -2}, vec3{1, 1, 1}, cube)
	if !ok || !vecClose(h.point, vec3{-1, -1, -1}) {
		t.Fatalf("tie hit ok=%v point=%v", ok, h.point)
	}
	if !vecClose(h.normal, vec3{-1, 0, 0}) {
		t.Errorf("tie normal %v, want -x (first axis wins)", h.normal)
	}
}

func TestNearestHitOrdering(t *testing.T) {
	near := sphere(vec3{0, 0, 0}, 1)
	far := sphere(vec3{0, 0, -3}, 1)
	origin, dir := vec3{0, 0, 3}, vec3{0, 0, -1}

	found := false
	var nearest rayHit
	for _, p := range []part{near, far} {
		if h, ok := intersectEllipsoid(origin, dir, p); ok && (!found || h.t < nearest.t) {
			nearest = h
			found = true
		}
	}
	if !found || !vecClose(nearest.point, vec3{0, 0, 1}) {
		t.Errorf("nearest hit = %v, want the near sphere front", nearest.point)
	}
}

func TestMaterialEyeLidPupilBoundary(t *testing.T) {
	eye := rigParts[4] // eye-left
	rot := newRotation(0, 0)
	point := func(u, v float64) rayHit {
		return rayHit{
			point:  vec3{eye.center.x + u*eye.radii.x, eye.center.y + v*eye.radii.y, eye.center.z},
			normal: vec3{0, 0, 1},
		}
	}

	// EyeOpen 0.5 puts the lid threshold at v = 0; the comparison is strict.
	pose := Pose{EyeOpen: 0.5, Lift: 0.06}
	if got := material(point(0.5, 0), eye, pose, rot); got.role != roleEye {
		t.Errorf("v at threshold should still be eye, got %s", roleName(got.role))
	}
	if got := material(point(0.5, 1e-6), eye, pose, rot); got.role != roleFur {
		t.Errorf("v just above threshold should be lid/fur, got %s", roleName(got.role))
	}
	if got := material(point(0.5, -1e-6), eye, pose, rot); got.role != roleEye {
		t.Errorf("v just below threshold should be eye, got %s", roleName(got.role))
	}

	// Pupil is a unit disc around (pupilU, -0.10); the boundary is inclusive.
	pupilU := 0.06 // -sign(-0.37) * 0.06
	if got := material(point(pupilU, -0.10), eye, pose, rot); got.color != 7 || got.role != rolePupil {
		t.Errorf("pupil center: color=%d role=%s", got.color, roleName(got.role))
	}
	if got := material(point(pupilU+0.18, -0.10), eye, pose, rot); got.color != 7 || got.role != rolePupil {
		t.Errorf("pupil boundary should still be pupil, got color=%d role=%s", got.color, roleName(got.role))
	}
	if got := material(point(pupilU+0.1801, -0.10), eye, pose, rot); got.role != roleEye || got.color != 6 {
		t.Errorf("just outside pupil should be white eye, got color=%d role=%s", got.color, roleName(got.role))
	}
	// Right eye mirrors the pupil horizontally.
	right := rigParts[5]
	rp := rayHit{point: vec3{right.center.x - 0.06*right.radii.x, right.center.y - 0.10*right.radii.y, right.center.z}, normal: vec3{0, 0, 1}}
	if got := material(rp, right, pose, rot); got.role != rolePupil {
		t.Errorf("right pupil mirror: role=%s", roleName(got.role))
	}
}

func TestMaterialMouthBoundary(t *testing.T) {
	head := rigParts[1]
	rot := newRotation(0, 0)
	pose := Pose{Lift: 0.06}
	pointAt := func(x, y float64, nz float64) rayHit {
		return rayHit{point: vec3{x, y, 1}, normal: vec3{0, 0, nz}}
	}
	mouthY := func(x float64) float64 {
		t := clamp((math.Abs(x)-0.15)/0.09, 0, 1)
		return -0.195 + pose.Lift*t*t
	}

	if got := material(pointAt(0.20, mouthY(0.20), 1), head, pose, rot); got.role != roleMouth || got.color != 7 {
		t.Errorf("inside mouth band: color=%d role=%s", got.color, roleName(got.role))
	}
	// Half-thickness is .030 inclusive; just past it is fur again.
	if got := material(pointAt(0.20, mouthY(0.20)+0.030, 1), head, pose, rot); got.role != roleMouth {
		t.Errorf("mouth band edge should be inclusive, got %s", roleName(got.role))
	}
	if got := material(pointAt(0.20, mouthY(0.20)+0.031, 1), head, pose, rot); got.role == roleMouth {
		t.Error("past the band should not be mouth")
	}
	// |x| must stay within .24.
	if got := material(pointAt(0.241, mouthY(0.24), 1), head, pose, rot); got.role == roleMouth {
		t.Error("|x|>0.24 should not be mouth")
	}
	// The local normal must face forward.
	if got := material(pointAt(0.20, mouthY(0.20), -1), head, pose, rot); got.role == roleMouth {
		t.Error("back-facing normal should not be mouth")
	}
	// The muzzle shares the rule; the nose does not.
	if got := material(pointAt(0.20, mouthY(0.20), 1), rigParts[6], pose, rot); got.role != roleMouth {
		t.Error("muzzle should carry the mouth rule")
	}
	if got := material(pointAt(0.20, mouthY(0.20), 1), rigParts[7], pose, rot); got.role != roleNose {
		t.Error("nose should not carry the mouth rule")
	}
}

func TestToothSeparation(t *testing.T) {
	left, right := rigParts[8], rigParts[9]
	dir := vec3{0, 0, -1}

	// The gap between the teeth: a ray down the centre hits neither box.
	if _, ok := intersectBox(vec3{0, -0.34, 3}, dir, left); ok {
		t.Error("centre ray should miss the left tooth")
	}
	if _, ok := intersectBox(vec3{0, -0.34, 3}, dir, right); ok {
		t.Error("centre ray should miss the right tooth")
	}
	// A ray through each tooth centre hits only that tooth.
	if _, ok := intersectBox(vec3{0.09, -0.34, 3}, dir, left); ok {
		t.Error("x=0.09 should miss the left tooth")
	}
	if _, ok := intersectBox(vec3{0.09, -0.34, 3}, dir, right); !ok {
		t.Error("x=0.09 should hit the right tooth")
	}
	if _, ok := intersectBox(vec3{-0.09, -0.34, 3}, dir, right); ok {
		t.Error("x=-0.09 should miss the right tooth")
	}
	if _, ok := intersectBox(vec3{-0.09, -0.34, 3}, dir, left); !ok {
		t.Error("x=-0.09 should hit the left tooth")
	}
}

// TestSceneTeethLeaveAGap confirms the separation survives sampling: across the
// tooth rows there are tooth samples on both sides and a non-tooth column
// between them at the centre.
func TestSceneTeethLeaveAGap(t *testing.T) {
	var samples [SampleWidth * SampleHeight]sample
	sampleScene(NeutralPose(), &samples)
	centre := 0
	left, right := 0, 0
	// World x=0 maps to sample column 31.5, so sx=31 and sx=32 straddle centre.
	for sy := 0; sy < SampleHeight; sy++ {
		if samples[sy*SampleWidth+31].role == roleTooth || samples[sy*SampleWidth+32].role == roleTooth {
			centre++
		}
		for sx := 28; sx <= 30; sx++ {
			if samples[sy*SampleWidth+sx].role == roleTooth {
				left++
			}
		}
		for sx := 33; sx <= 35; sx++ {
			if samples[sy*SampleWidth+sx].role == roleTooth {
				right++
			}
		}
	}
	if left == 0 || right == 0 {
		t.Fatalf("expected tooth samples on both sides, got left=%d right=%d", left, right)
	}
	if centre != 0 {
		t.Errorf("expected a gap at the centre columns, found %d tooth samples", centre)
	}
}
