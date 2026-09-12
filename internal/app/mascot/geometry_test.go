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

func TestPlateFlatRoundedAndParallel(t *testing.T) {
	plate := rigParts[16] // tooth-left: corner 0.035
	dir := vec3{0, 0, -1}

	// Through the centre: hit the flat +z face.
	h, ok := intersectPlate(vec3{plate.center.x, plate.center.y, 3}, dir, plate)
	if !ok || !vecClose(h.point, vec3{plate.center.x, plate.center.y, plate.center.z + plate.radii.z}) {
		t.Errorf("plate front hit ok=%v point=%v", ok, h.point)
	}
	if !vecClose(h.normal, vec3{0, 0, 1}) {
		t.Errorf("plate normal %v, want +z", h.normal)
	}
	// Just beyond the half-extent on x: miss (outside the rounded corner).
	if _, ok := intersectPlate(vec3{plate.center.x + plate.radii.x + 0.001, plate.center.y, 3}, dir, plate); ok {
		t.Error("ray outside the plate should miss")
	}
	// A ray parallel to z (within the threshold) misses.
	if _, ok := intersectPlate(vec3{plate.center.x, plate.center.y, 3}, vec3{0, 0, -1e-11}, plate); ok {
		t.Error("near-parallel plate ray should miss")
	}
	// A slightly slanted ray still crosses the flat face inside the plate.
	if _, ok := intersectPlate(vec3{plate.center.x, plate.center.y, 3}, vec3{0.02, 0, -1}, plate); !ok {
		t.Error("slanted ray through the plate should hit")
	}
}

func TestNearestHitOrdering(t *testing.T) {
	near := sphere(vec3{0, 0, 0}, 1)
	far := sphere(vec3{0, 0, -3}, 1)
	origin, dir := vec3{0, 0, 3}, vec3{0, 0, -1}

	found := false
	var nearest rayHit
	for _, p := range []part{near, far} {
		if h, ok := intersect(origin, dir, p); ok && (!found || h.t < nearest.t) {
			nearest = h
			found = true
		}
	}
	if !found || !vecClose(nearest.point, vec3{0, 0, 1}) {
		t.Errorf("nearest hit = %v, want the near sphere front", nearest.point)
	}
}

// TestMaterialRoleColors pins the flat palette region each role paints.
func TestMaterialRoleColors(t *testing.T) {
	rot := newRotation(0, 0)
	hit := rayHit{point: vec3{0, 0, 1}, normal: vec3{0, 0, 1}}
	for _, tc := range []struct {
		p    part
		want uint8
	}{
		{part{role: roleNose}, 7},
		{part{role: roleEar}, 7},
		{part{role: roleMouth}, 7},
		{part{role: roleSeam}, 7},
		{part{role: roleMuzzle}, 8},
		{part{role: roleTooth}, 6},
	} {
		if got := material(hit, tc.p, Pose{}, rot); got.color != tc.want || got.role != tc.p.role {
			t.Errorf("material(%s) = color %d role %s, want color %d", roleName(tc.p.role), got.color, roleName(got.role), tc.want)
		}
	}
}

func TestMaterialEyeLidPupilBoundary(t *testing.T) {
	eye := rigParts[7] // eye-left
	rot := newRotation(0, 0)
	point := func(y float64) rayHit {
		return rayHit{point: vec3{eye.center.x, y, eye.center.z}, normal: vec3{0, 0, 1}}
	}

	// Wide-open: lid sits above the eye, so the ivory reads as eye white.
	open := Pose{EyeOpen: 1, Lift: 0.06}
	if got := material(point(eye.center.y), eye, open, rot); got.role != roleEye || got.color != 6 {
		t.Errorf("open eye: color=%d role=%s, want 6/eye", got.color, roleName(got.role))
	}
	// Closed: lid drops to the eye's bottom, covering white with shaded fur.
	closed := Pose{EyeOpen: 0, Lift: 0.06}
	if got := material(point(eye.center.y), eye, closed, rot); got.role != roleFur {
		t.Errorf("closed eye should be shaded fur, got %s", roleName(got.role))
	}
	// Threshold at EyeOpen 0.5 is the eye centre; the comparison is strict.
	half := Pose{EyeOpen: 0.5, Lift: 0.06}
	if got := material(point(eye.center.y), eye, half, rot); got.role != roleEye {
		t.Errorf("point at the lid threshold should still be eye, got %s", roleName(got.role))
	}
	if got := material(point(eye.center.y+1e-6), eye, half, rot); got.role != roleFur {
		t.Errorf("point just above the lid should be fur, got %s", roleName(got.role))
	}

	// The pupil is its own ellipsoid; it reads dark until the lid covers it.
	pupil := rigParts[9] // pupil-left
	pp := func(y float64) rayHit {
		return rayHit{point: vec3{pupil.center.x, y, pupil.center.z}, normal: vec3{0, 0, 1}}
	}
	if got := material(pp(pupil.center.y), pupil, open, rot); got.role != rolePupil || got.color != 7 {
		t.Errorf("open pupil: color=%d role=%s, want 7/pupil", got.color, roleName(got.role))
	}
	if got := material(pp(pupil.center.y), pupil, closed, rot); got.role != roleFur {
		t.Errorf("closed pupil should be shaded fur, got %s", roleName(got.role))
	}
}

// TestSceneTeethLeaveAGap confirms the paired teeth render on both sides of the
// divider with no tooth role in the centre columns.
func TestSceneTeethLeaveAGap(t *testing.T) {
	var samples [SampleWidth * SampleHeight]sample
	sampleScene(NeutralPose(), &samples)

	left, right, centre := 0, 0, 0
	for sy := 0; sy < SampleHeight; sy++ {
		for sx := 0; sx < SampleWidth; sx++ {
			if samples[sy*SampleWidth+sx].role != roleTooth {
				continue
			}
			switch {
			case sx >= 56 && sx <= 63:
				left++
			case sx >= 66 && sx <= 71:
				right++
			case sx == 64 || sx == 65:
				centre++
			}
		}
	}
	if left == 0 || right == 0 {
		t.Fatalf("expected tooth samples on both sides, got left=%d right=%d", left, right)
	}
	if centre != 0 {
		t.Errorf("expected no tooth samples at the divider columns, found %d", centre)
	}
}
