package mascot

import (
	"math"
	"testing"
	"testing/synctest"
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
	m.moving = true
	m.setTarget(target)
	m.pending = true
	m.seq = 1
	m.nextDeadline = t0.Add(frameInterval)
	return m
}

func TestModelArmsOneChain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := New()
		_, cmd := m.Configure(visibleScene(), time.Now())
		if cmd == nil {
			t.Fatal("first visible Configure must arm")
		}
		if _, extra := m.Configure(visibleScene(), time.Now()); extra != nil {
			t.Error("Configure while pending must not arm another chain")
		}
		msg, ok := cmd().(FrameMsg)
		if !ok {
			t.Fatal("clock command must return FrameMsg")
		}
		if changed, next := m.Update(msg, time.Now()); changed || next == nil {
			t.Error("settled frame must keep one clock chain without changing the art")
		}
	})
}

// TestModelRejectsWrongOwnerSeqDuplicate checks frame validation: a foreign
// owner, a stale sequence and a duplicate never reschedule.
func TestModelRejectsWrongOwnerSeqDuplicate(t *testing.T) {
	m, other := New(), New()
	t0 := time.Unix(1000, 0)
	m.Configure(visibleScene(), t0)
	other.Configure(visibleScene(), t0)
	seq := m.seq

	if _, cmd := m.Update(FrameMsg{owner: other, seq: other.seq}, t0); cmd != nil {
		t.Error("foreign owner must be rejected")
	}
	if !m.pending {
		t.Error("rejected frame must keep the tick pending")
	}
	if _, cmd := m.Update(FrameMsg{owner: m, seq: seq + 7}, t0); cmd != nil {
		t.Error("stale sequence must be rejected")
	}

	at := t0.Add(frameInterval)
	if _, cmd := m.Update(FrameMsg{owner: m, seq: seq}, at); cmd == nil {
		t.Fatal("a valid frame must arm its successor")
	}
	if m.seq != seq+1 {
		t.Errorf("sequence advanced to %d, want %d", m.seq, seq+1)
	}
	if _, cmd := m.Update(FrameMsg{owner: m, seq: seq}, t0); cmd != nil {
		t.Error("a duplicate frame must not reschedule")
	}
}

// TestModelHideShowInvalidatesStale checks that hiding via a zero-size slot is
// idempotent and any in-flight timer is discarded, while showing starts a fresh
// chain.
func TestModelHideShowInvalidatesStale(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.Configure(visibleScene(), t0)
	stale := m.seq

	hidden := Scene{}
	m.Configure(hidden, t0)
	if m.visible || m.pending {
		t.Error("hiding must clear visibility and the pending tick")
	}
	if m.View() != "" {
		t.Error("a hidden model must render nothing")
	}
	hiddenSeq := m.seq
	m.Configure(hidden, t0)
	if m.seq != hiddenSeq {
		t.Error("repeated hiding must be idempotent")
	}
	if _, cmd := m.Update(FrameMsg{owner: m, seq: stale}, t0); cmd != nil {
		t.Error("a stale tick during hide must be ignored")
	}

	t1 := t0.Add(time.Second)
	if _, cmd := m.Configure(visibleScene(), t1); cmd == nil {
		t.Error("showing again must arm a fresh chain")
	}
	if changed, cmd := m.Update(FrameMsg{owner: m, seq: stale}, t1); changed || cmd != nil || !m.pending {
		t.Error("pre-hide tick must not disturb the fresh chain after showing")
	}
}

// TestModelStallSkipsSlots checks the deadline advances by whole intervals to a
// strictly future time after a long handling stall, never replaying slots.
func TestModelStallSkipsSlots(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.Configure(visibleScene(), t0)

	handling := t0.Add(10 * frameInterval)
	if _, cmd := m.Update(FrameMsg{owner: m, seq: m.seq}, handling); cmd == nil {
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
		if m.pose != sanitizePose(m.pose) {
			t.Fatalf("nonfinite or out-of-bounds pose: %+v", m.pose)
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
	m.moving = false
	m.manualAt = t0

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

// TestSceneTargetMapsSelection checks the target-to-pose mapping: independent
// axis normalization, correct yaw sign, a useful pitch range and clamping.
func TestSceneTargetMapsSelection(t *testing.T) {
	scene := Scene{
		Slot:    ui.Rect{X: 24, Y: 1, Width: 32, Height: 12},
		Content: ui.Rect{X: 28, Y: 14, Width: 24, Height: 10},
		Track:   true,
	}

	center := scene
	center.Target = ui.Point{X: 40, Y: 14}
	if got := sceneTarget(center); got.Yaw != 0 || got.Pitch != pitchBase {
		t.Errorf("center target = %+v, want yaw 0 pitch %v", got, pitchBase)
	}

	right := scene
	right.Target = ui.Point{X: 52, Y: 19}
	if got := sceneTarget(right); got.Yaw != yawRange || got.Pitch != pitchBase+pitchRange/2 {
		t.Errorf("right target = %+v, want yaw %v pitch %v", got, yawRange, pitchBase+pitchRange/2)
	}

	left := scene
	left.Target = ui.Point{X: 28, Y: 14}
	if got := sceneTarget(left); got.Yaw != -yawRange || got.Pitch != pitchBase {
		t.Errorf("left target = %+v, want yaw %v pitch %v", got, -yawRange, pitchBase)
	}

	clamped := scene
	clamped.Target = ui.Point{X: 400, Y: 400}
	if got := sceneTarget(clamped); got.Yaw != yawRange || got.Pitch != pitchBase+pitchRange {
		t.Errorf("out-of-range target = %+v, want clamped yaw %v pitch %v", got, yawRange, pitchBase+pitchRange)
	}
}

// TestSceneTargetNeutralWhenUntracked checks untracked or hidden scenes keep
// the neutral resting pose.
func TestSceneTargetNeutralWhenUntracked(t *testing.T) {
	untracked := Scene{
		Slot:    ui.Rect{X: 24, Y: 1, Width: 32, Height: 12},
		Content: ui.Rect{X: 28, Y: 14, Width: 24, Height: 10},
		Target:  ui.Point{X: 80, Y: 30},
	}
	if got := sceneTarget(untracked); got != NeutralPose() {
		t.Errorf("untracked scene = %+v, want neutral", got)
	}

	hidden := untracked
	hidden.Track = true
	hidden.Slot = ui.Rect{}
	if got := sceneTarget(hidden); got != NeutralPose() {
		t.Errorf("hidden scene = %+v, want neutral", got)
	}
}

// TestModelSleepyThenWake checks the play clock drives a sleepy base after a
// pause and an eligible wake starts calm, folding both into the pose target.
func TestModelSleepyThenWake(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.ObserveAttempt(okAt(t0))
	m.Configure(visibleScene(), t0)

	sleep := t0.Add(sleepAfter)
	m.Update(FrameMsg{owner: m, seq: m.seq}, sleep)
	if m.target.EyeOpen != sleepyMood.EyeOpen || m.target.Lift != sleepyMood.Lift {
		t.Errorf("sleepy target = %+v, want %+v", m.target, sleepyMood)
	}

	wake := sleep.Add(500 * time.Millisecond)
	m.manualAt = wake
	m.Activity(wake)
	m.Configure(visibleScene(), wake)
	if m.target.EyeOpen != calmMood.EyeOpen || m.target.Lift != calmMood.Lift {
		t.Errorf("wake target = %+v, want calm %+v", m.target, calmMood)
	}
	if !m.manualAt.IsZero() {
		t.Error("wake must cancel an in-progress blink")
	}
}

// TestModelExcitedFoldsBaseAndBob checks sustained fast accurate evidence
// raises the excited base and its restrained bob target, then settling leaves
// the eye and lift at the excited preset.
func TestModelExcitedFoldsBaseAndBob(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	for i := 0; i < 20; i++ {
		m.ObserveAttempt(paceAt(t0.Add(time.Duration(i)*100*time.Millisecond), true))
	}
	arm := t0.Add(1900 * time.Millisecond)
	m.Configure(visibleScene(), arm)

	base := arm.Add(moodHold)
	m.Update(FrameMsg{owner: m, seq: m.seq}, base)
	if m.target.EyeOpen != excitedMood.EyeOpen || m.target.Lift != excitedMood.Lift {
		t.Fatalf("excited base not folded, target = %+v", m.target)
	}

	quarter := base.Add(125 * time.Millisecond)
	m.Update(FrameMsg{owner: m, seq: m.seq}, quarter)
	if math.Abs(m.target.Bob-excitedBobAmp) > 1e-9 {
		t.Errorf("excited bob target = %g, want %g", m.target.Bob, excitedBobAmp)
	}
}

// TestConfigureTracksWithoutTeleport checks Configure adopts the scene target,
// snaps on first show and preserves the visible pose when retargeting.
func TestConfigureTracksWithoutTeleport(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	scene := Scene{
		Slot:    ui.Rect{X: 24, Y: 1, Width: 32, Height: 12},
		Content: ui.Rect{X: 28, Y: 14, Width: 24, Height: 10},
		Target:  ui.Point{X: 52, Y: 24},
		Track:   true,
	}
	m.Configure(scene, t0)
	if m.target.Yaw != 35 || m.target.Pitch != 20 {
		t.Errorf("target = %+v, want yaw 35 pitch 20", m.target)
	}
	if m.pose != m.target {
		t.Errorf("first show must snap to target, pose %+v", m.pose)
	}

	scene.Target = ui.Point{X: 28, Y: 14}
	m.Configure(scene, t0.Add(frameInterval))
	if m.pose.Yaw != 35 {
		t.Errorf("retarget must not teleport, pose = %+v", m.pose)
	}
	if m.target.Yaw != -35 {
		t.Errorf("retarget target = %+v, want yaw -35", m.target)
	}
}
