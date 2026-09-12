package mascot

import (
	"testing"
	"time"

	"gopherttype/internal/app/ui"
)

func visibleScene() Scene {
	return Scene{Slot: ui.Rect{X: 0, Y: 1, Width: Width, Height: Height}}
}

// motionModel builds a visible, moving model with a non-neutral target and a
// synthetic pending first tick so tests can drive frames without real timers.
func motionModel(t0 time.Time, target Pose) *Model {
	m := New()
	m.setVisible(true, t0)
	m.setMoving(true)
	m.setTarget(target)
	m.pending = true
	m.seq = 1
	m.nextDeadline = t0.Add(frameInterval)
	return m
}

// TestModelArmsOneChain checks repeated visible configuration arms exactly one
// successor.
func TestModelArmsOneChain(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	if _, cmd := m.Configure(visibleScene(), t0); cmd == nil {
		t.Fatal("first visible Configure must arm")
	}
	if !m.pending {
		t.Fatal("arming must mark pending")
	}
	if _, cmd := m.Configure(visibleScene(), t0.Add(time.Millisecond)); cmd != nil {
		t.Error("Configure while pending must not arm another chain")
	}
}

// TestModelRejectsWrongOwnerSeqDuplicate checks frame validation: a foreign
// owner, a stale sequence and a duplicate never reschedule.
func TestModelRejectsWrongOwnerSeqDuplicate(t *testing.T) {
	m, other := New(), New()
	t0 := time.Unix(1000, 0)
	m.Configure(visibleScene(), t0)
	seq := m.seq

	if _, cmd := m.Update(FrameMsg{owner: other, seq: seq, at: t0}, t0); cmd != nil {
		t.Error("foreign owner must be rejected")
	}
	if !m.pending {
		t.Error("rejected frame must keep the tick pending")
	}
	if _, cmd := m.Update(FrameMsg{owner: m, seq: seq + 7, at: t0}, t0); cmd != nil {
		t.Error("stale sequence must be rejected")
	}

	at := t0.Add(frameInterval)
	if _, cmd := m.Update(FrameMsg{owner: m, seq: seq, at: at}, at); cmd == nil {
		t.Fatal("a valid frame must arm its successor")
	}
	if m.seq != seq+1 {
		t.Errorf("sequence advanced to %d, want %d", m.seq, seq+1)
	}
	if _, cmd := m.Update(FrameMsg{owner: m, seq: seq, at: t0}, t0); cmd != nil {
		t.Error("a duplicate frame must not reschedule")
	}
}

// TestModelHideShowInvalidatesStale checks Hide is idempotent and any in-flight
// timer is discarded, while showing starts a fresh chain.
func TestModelHideShowInvalidatesStale(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.Configure(visibleScene(), t0)
	stale := m.seq

	m.Hide()
	if m.visible || m.pending {
		t.Error("Hide must clear visibility and the pending tick")
	}
	if m.View() != "" {
		t.Error("a hidden model must render nothing")
	}
	hiddenSeq := m.seq
	m.Hide()
	if m.seq != hiddenSeq {
		t.Error("repeated Hide must be idempotent")
	}
	if _, cmd := m.Update(FrameMsg{owner: m, seq: stale, at: t0}, t0); cmd != nil {
		t.Error("a stale tick during hide must be ignored")
	}

	t1 := t0.Add(time.Second)
	if _, cmd := m.Configure(visibleScene(), t1); cmd == nil {
		t.Error("showing again must arm a fresh chain")
	}
}

// TestModelSecondModelSameSeq isolates owners: two models with matching
// sequences must not accept each other's frames.
func TestModelSecondModelSameSeq(t *testing.T) {
	a, b := New(), New()
	t0 := time.Unix(1000, 0)
	a.Configure(visibleScene(), t0)
	b.Configure(visibleScene(), t0)
	if a.seq != b.seq {
		t.Fatalf("precondition: seq %d vs %d", a.seq, b.seq)
	}
	if _, cmd := a.Update(FrameMsg{owner: b, seq: a.seq, at: t0}, t0); cmd != nil {
		t.Error("a frame owned by another model must be rejected")
	}
}

// TestModelStallSkipsSlots checks the deadline advances by whole intervals to a
// strictly future time after a long handling stall, never replaying slots.
func TestModelStallSkipsSlots(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.Configure(visibleScene(), t0)

	handling := t0.Add(10 * frameInterval)
	fired := t0.Add(frameInterval)
	if _, cmd := m.Update(FrameMsg{owner: m, seq: m.seq, at: fired}, handling); cmd == nil {
		t.Fatal("stalled frame must still arm")
	}
	delay := m.nextDeadline.Sub(handling)
	if delay <= 0 || delay > frameInterval {
		t.Errorf("post-stall delay = %v, want (0,%v]", delay, frameInterval)
	}
}

// TestModelFirstStepUsesOneInterval checks the first step after showing uses one
// frame interval regardless of how much wall time elapsed before it.
func TestModelFirstStepUsesOneInterval(t *testing.T) {
	t0 := time.Unix(1000, 0)
	target := Pose{Yaw: 30, Pitch: 10, EyeOpen: 0.6, Lift: 0.05, Bob: 0.03}

	a := motionModel(t0, target)
	a.advancePose(t0.Add(frameInterval))

	b := motionModel(t0, target)
	b.advancePose(t0.Add(10 * time.Second))

	if a.pose != b.pose {
		t.Errorf("first step must be one interval: %+v vs %+v", a.pose, b.pose)
	}
}

// TestModelStallDtClamp checks a >50 ms gap advances the springs only 50 ms.
func TestModelStallDtClamp(t *testing.T) {
	t0 := time.Unix(1000, 0)
	target := Pose{Yaw: 30, Pitch: 10, EyeOpen: 0.6, Lift: 0.05, Bob: 0.03}
	base := t0.Add(frameInterval)

	a := motionModel(t0, target)
	a.advancePose(base)
	a.advancePose(base.Add(50 * time.Millisecond))

	b := motionModel(t0, target)
	b.advancePose(base)
	b.advancePose(base.Add(200 * time.Millisecond))

	if a.pose != b.pose {
		t.Errorf("dt must clamp at 50 ms: %+v vs %+v", a.pose, b.pose)
	}
}

// TestModelZeroAndReversedDt checks a zero or reversed delta skips spring math.
func TestModelZeroAndReversedDt(t *testing.T) {
	t0 := time.Unix(1000, 0)
	m := motionModel(t0, Pose{Yaw: 30, Pitch: 10, EyeOpen: 0.6, Lift: 0.05, Bob: 0.03})
	m.advancePose(t0.Add(frameInterval))
	settled := m.pose

	m.advancePose(t0.Add(frameInterval)) // zero delta
	if m.pose != settled {
		t.Error("zero delta must skip spring math")
	}
	m.advancePose(t0) // reversed
	if m.pose != settled {
		t.Error("reversed delta must skip spring math")
	}
}

// TestModelSpringConvergesAndBounded checks the springs reach the target,
// settle with zero velocity and never leave renderer bounds.
func TestModelSpringConvergesAndBounded(t *testing.T) {
	t0 := time.Unix(1000, 0)
	target := Pose{Yaw: 30, Pitch: 15, EyeOpen: 0.5, Lift: 0.02, Bob: 0.04}
	m := motionModel(t0, target)

	now := t0.Add(frameInterval)
	for i := 0; i < 3000; i++ {
		m.advancePose(now)
		if m.pose.Yaw < -35 || m.pose.Yaw > 35 {
			t.Fatalf("yaw out of bounds: %g", m.pose.Yaw)
		}
		if m.pose.Pitch < 0 || m.pose.Pitch > 20 {
			t.Fatalf("pitch out of bounds: %g", m.pose.Pitch)
		}
		if m.pose.EyeOpen < 0 || m.pose.EyeOpen > 1 {
			t.Fatalf("eye out of bounds: %g", m.pose.EyeOpen)
		}
		if m.pose.Bob < -0.06 || m.pose.Bob > 0.06 {
			t.Fatalf("bob out of bounds: %g", m.pose.Bob)
		}
		now = now.Add(frameInterval)
	}
	if want := sanitizePose(target); m.pose != want {
		t.Errorf("did not settle: got %+v, want %+v", m.pose, want)
	}
	if m.vel != (poseVel{}) {
		t.Errorf("settled velocity = %+v, want zero", m.vel)
	}
}

// TestModelYawSign checks left/right targets turn the head the right way.
func TestModelYawSign(t *testing.T) {
	t0 := time.Unix(1000, 0)
	right := motionModel(t0, Pose{Yaw: 30})
	right.advancePose(t0.Add(frameInterval))
	if right.pose.Yaw <= 0 {
		t.Errorf("positive target should turn positive, got %g", right.pose.Yaw)
	}

	left := motionModel(t0, Pose{Yaw: -30})
	left.advancePose(t0.Add(frameInterval))
	if left.pose.Yaw >= 0 {
		t.Errorf("negative target should turn negative, got %g", left.pose.Yaw)
	}
}

// TestModelAutomaticBlinkSchedule checks the first automatic blink is at 4 s of
// visible motion, closes fully at its midpoint and covers only 140 ms.
func TestModelAutomaticBlinkSchedule(t *testing.T) {
	t0 := time.Unix(1000, 0)
	m := motionModel(t0, NeutralPose())

	if f := m.blinkFactor(t0.Add(2 * time.Second)); f != 1 {
		t.Errorf("no blink before 4 s, factor %g", f)
	}
	mid := t0.Add(blinkPeriod + blinkDuration/2)
	if f := m.blinkFactor(mid); f > 0.05 {
		t.Errorf("mid-blink factor %g, want near 0", f)
	}
	after := t0.Add(blinkPeriod + blinkDuration)
	if f := m.blinkFactor(after); f != 1 {
		t.Errorf("blink must end at 140 ms, factor %g", f)
	}
}

// TestModelHeldManualBlink checks a manual blink works while held and clears
// itself when complete.
func TestModelHeldManualBlink(t *testing.T) {
	t0 := time.Unix(1000, 0)
	m := New()
	m.setVisible(true, t0)
	m.setMoving(false)
	m.startBlink(t0)

	if !m.manualBlinkActive(t0.Add(blinkDuration / 2)) {
		t.Error("manual blink should be active mid-envelope")
	}
	if f := m.blinkFactor(t0.Add(blinkDuration / 2)); f > 0.05 {
		t.Errorf("manual mid-blink factor %g, want near 0", f)
	}
	m.blinkFactor(t0.Add(blinkDuration))
	if m.manualBlinkActive(t0.Add(blinkDuration)) {
		t.Error("manual blink should complete after 140 ms")
	}
	if m.armNext(t0.Add(blinkDuration)) != nil {
		t.Error("a completed held blink must not re-arm")
	}
}

// TestModelHiddenNoWork checks a hidden model renders nothing and arms nothing.
func TestModelHiddenNoWork(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	if m.View() != "" {
		t.Error("hidden model must have an empty view")
	}
	if m.render(t0) {
		t.Error("hidden render must be a no-op")
	}
	if m.armNext(t0) != nil {
		t.Error("hidden model must not arm a tick")
	}
}

// TestModelSettledKeepsCadence checks a settled visible component still arms
// the next blink deadline even when the pose did not change.
func TestModelSettledKeepsCadence(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.Configure(visibleScene(), t0)
	at := t0.Add(frameInterval)
	changed, cmd := m.Update(FrameMsg{owner: m, seq: m.seq, at: at}, at)
	if changed {
		t.Error("a settled neutral frame should not change the art")
	}
	if cmd == nil {
		t.Error("a settled visible component must keep the cadence")
	}
}
