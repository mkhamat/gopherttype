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
	v := vec3{-0.3, 0.7, 0.2}
	for _, tc := range []struct{ yaw, pitch float64 }{{0, 0}, {35, 20}, {-35, 0}, {12.34, 7.77}} {
		rot := newRotation(tc.yaw, tc.pitch)
		if got := rot.inverse(rot.forward(v)); !vecClose(got, v) {
			t.Errorf("yaw %g pitch %g: inverse(forward(%v)) = %v", tc.yaw, tc.pitch, v, got)
		}
		if got := rot.forward(rot.inverse(v)); !vecClose(got, v) {
			t.Errorf("yaw %g pitch %g: forward(inverse(%v)) = %v", tc.yaw, tc.pitch, v, got)
		}
	}
	if got := newRotation(30, 0).forward(vec3{1, 0, 0}); got.x <= 0 || got.z >= 0 {
		t.Errorf("positive yaw forward(1,0,0) = %v, want +x/-z", got)
	}
	if got := newRotation(0, 30).forward(vec3{0, 1, 0}); got.y <= 0 || got.z <= 0 {
		t.Errorf("positive pitch forward(0,1,0) = %v, want +y/+z", got)
	}
}

func TestEllipsoidIntersections(t *testing.T) {
	p := part{radii: vec3{1, 1, 1}}
	for _, tc := range []struct {
		name                     string
		origin, direction, point vec3
		hit                      bool
	}{
		{"miss", vec3{2, 0, 3}, vec3{0, 0, -1}, vec3{}, false},
		{"front", vec3{0, 0, 3}, vec3{0, 0, -1}, vec3{0, 0, 1}, true},
		{"tangent", vec3{1, 0, 3}, vec3{0, 0, -1}, vec3{1, 0, 0}, true},
		{"past tangent", vec3{1.000001, 0, 3}, vec3{0, 0, -1}, vec3{}, false},
		{"inside", vec3{}, vec3{0, 0, -1}, vec3{0, 0, -1}, true},
		{"behind", vec3{0, 0, -3}, vec3{0, 0, -1}, vec3{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, ok := intersectEllipsoid(tc.origin, tc.direction, p)
			if ok != tc.hit {
				t.Fatalf("hit=%t, want %t", ok, tc.hit)
			}
			if ok && (h.t <= 0 || !vecClose(h.point, tc.point) || !vecClose(h.normal, tc.point)) {
				t.Errorf("hit %+v, want point/normal %v and positive t", h, tc.point)
			}
		})
	}
	unit, ok1 := intersectEllipsoid(vec3{0, 0, 3}, vec3{0, 0, -1}, p)
	scaled, ok2 := intersectEllipsoid(vec3{0, 0, 3}, vec3{0, 0, -2}, p)
	if !ok1 || !ok2 || !vecClose(unit.point, scaled.point) || math.Abs(unit.t-2*scaled.t) > 1e-9 {
		t.Errorf("direction scaling must preserve point and scale t: %+v vs %+v", unit, scaled)
	}
}

func TestPlateFlatRoundedAndParallel(t *testing.T) {
	plate := rigParts[16] // tooth-left: corner 0.035
	dir := vec3{0, 0, -1}
	h, ok := intersectPlate(vec3{plate.center.x, plate.center.y, 3}, dir, plate)
	if !ok || !vecClose(h.point, vec3{plate.center.x, plate.center.y, plate.center.z + plate.radii.z}) || !vecClose(h.normal, vec3{0, 0, 1}) {
		t.Errorf("plate front hit ok=%v hit=%+v", ok, h)
	}
	if _, ok := intersectPlate(vec3{plate.center.x + plate.radii.x + 0.001, plate.center.y, 3}, dir, plate); ok {
		t.Error("ray outside the plate should miss")
	}
	if _, ok := intersectPlate(vec3{plate.center.x, plate.center.y, 3}, vec3{0, 0, -1e-11}, plate); ok {
		t.Error("near-parallel plate ray should miss")
	}
	if _, ok := intersectPlate(vec3{plate.center.x, plate.center.y, 3}, vec3{0.02, 0, -1}, plate); !ok {
		t.Error("slanted ray through the plate should hit")
	}
}

func TestMaterialEyeLidPupilBoundary(t *testing.T) {
	for _, index := range []int{7, 9} {
		p := rigParts[index]
		eye := rigParts[7]
		for _, tc := range []struct {
			openness, y float64
			covered     bool
		}{
			{1, p.center.y, false}, {0, p.center.y, true},
			{0.5, eye.center.y, false}, {0.5, eye.center.y + 1e-6, true},
		} {
			hit := rayHit{point: vec3{p.center.x, tc.y, p.center.z}, normal: vec3{0, 0, 1}}
			got := material(hit, p, Pose{EyeOpen: tc.openness, Lift: 0.06}, newRotation(0, 0))
			if tc.covered {
				if got.role != roleFur || got.color < 1 || got.color > 5 {
					t.Errorf("covered part %d must be shaded fur, got %+v", index, got)
				}
			} else {
				wantColor := uint8(6)
				if p.role == rolePupil {
					wantColor = 7
				}
				if got.role != p.role || got.color != wantColor {
					t.Errorf("open part %d: got %+v, want role %d color %d", index, got, p.role, wantColor)
				}
			}
		}
	}
}
