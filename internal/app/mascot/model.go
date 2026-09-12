package mascot

import (
	"image/color"
	"math"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/harmonica"

	"gopherttype/internal/app/ui"
)

// The component keeps one successor chain for the active visible mascot. All
// animation (whole-head turn, bob, eye/lift easing and blinks) is evaluated on
// this single one-shot clock; renderer caching makes a settled frame cheap but
// does not stop the target cadence needed for future blink deadlines.
const (
	frameInterval = time.Second / 120
	maxFrameStep  = 50 * time.Millisecond

	blinkPeriod   = 4 * time.Second
	blinkDuration = 140 * time.Millisecond

	// Snap thresholds: angles in degrees, other channels in world/aperture
	// units. Below both error and velocity the spring settles exactly.
	angleEps = 0.001
	valueEps = 0.0001
)

// Scene describes where the mascot may appear and what it should attend to. A
// zero-size Slot means hidden. When Track is set, Target is the rendered point
// the head turns toward; otherwise the pose stays neutral.
type Scene struct {
	Slot       ui.Rect
	Content    ui.Rect
	Target     ui.Point
	Track      bool
	Background color.Color
}

// FrameMsg is one delivered clock tick. Only the component constructs it: the
// fields are private so no caller can forge the owner or sequence.
type FrameMsg struct {
	owner *Model
	seq   uint64
	at    time.Time
}

// poseVel holds one velocity per pose channel; angles are degrees/second, the
// rest are world or aperture units/second.
type poseVel struct {
	yaw, pitch, bob, eye, lift float64
}

// Model is the concrete mascot component. It owns one renderer, the current
// and desired pose, spring velocities, visibility and the guarded one-shot
// clock. It is not a tea.Model; screens drive it and wrap its cached view.
type Model struct {
	renderer *Renderer

	pose   Pose
	target Pose
	vel    poseVel

	bg     color.Color
	moving bool

	visible   bool
	visibleAt time.Time
	manualAt  time.Time

	pending      bool
	seq          uint64
	nextDeadline time.Time
	lastHandled  time.Time

	reaction reaction
}

// New creates neutral state and a renderer but arms no timer.
func New() *Model {
	return &Model{
		renderer: NewRenderer(),
		pose:     NeutralPose(),
		target:   NeutralPose(),
		reaction: newReaction(),
	}
}

// Configure applies a scene, then arms at most one frame command. It does not
// step the springs: pose changes between ticks are handled by the clock, keys
// do not get an extra time step.
func (m *Model) Configure(scene Scene, at time.Time) (bool, tea.Cmd) {
	m.bg = scene.Background
	m.moving = true
	m.setTarget(sceneTarget(scene))
	m.applyReaction(at)

	visible := scene.Slot.Width > 0 && scene.Slot.Height > 0
	visChanged := m.setVisible(visible, at)
	renderChanged := m.render(at)
	return visChanged || renderChanged, m.armNext(at)
}

// Update validates one frame message and advances the animation toward the
// current target using the caller's handling time, not the timer timestamp.
func (m *Model) Update(msg FrameMsg, at time.Time) (bool, tea.Cmd) {
	if !m.validateFrame(msg) {
		return false, nil
	}
	m.applyReaction(at)
	changed := m.advancePose(at)
	return changed, m.armNext(at)
}

// applyReaction advances the play reaction machine and folds its base mood into
// the pose target. It performs no spring step; a flinch is applied at render.
func (m *Model) applyReaction(at time.Time) {
	m.reaction.step(at)
	if m.reaction.woke {
		m.reaction.woke = false
		m.cancelBlink(at)
	}
	m.target.EyeOpen = m.reaction.base.EyeOpen
	m.target.Lift = m.reaction.base.Lift
	m.target.Bob = m.reaction.bobTarget(at)
}

// cancelBlink drops any in-progress manual or automatic blink so a wake opens
// the eye immediately instead of stacking closures with the sleepy lids.
func (m *Model) cancelBlink(at time.Time) {
	if !m.manualAt.IsZero() && at.Sub(m.manualAt) < blinkDuration {
		m.manualAt = time.Time{}
	}
	if m.moving && m.visible && !m.visibleAt.IsZero() {
		if elapsed := at.Sub(m.visibleAt); elapsed >= blinkPeriod {
			if phase := (elapsed - blinkPeriod) % blinkPeriod; phase < blinkDuration {
				m.visibleAt = at
			}
		}
	}
}

// View returns the cached portrait, or empty while hidden. It never renders or
// reads time.
func (m *Model) View() string {
	if !m.visible {
		return ""
	}
	return m.renderer.View()
}

// setTarget stores a sanitized desired pose. Spring motion converges on it.
func (m *Model) setTarget(p Pose) {
	m.target = sanitizePose(p)
}

// Attention sensitivity. Yaw spans ±yawRange across half the content width;
// pitch rises from pitchBase at the content top by pitchRange over its full
// height. Tune these numbers, never the geometry.
const (
	yawRange   = 35.0
	pitchBase  = 5.0
	pitchRange = 15.0
)

// sceneTarget maps a scene's rendered target to a desired pose. Each axis is
// normalized independently from the slot and content geometry, so horizontal
// columns are never compared to vertical rows as equal distances. It is an
// artistic direction, not a physical gaze ray.
func sceneTarget(scene Scene) Pose {
	pose := NeutralPose()
	if !scene.Track || scene.Slot.Width <= 0 {
		return pose
	}
	headX := float64(scene.Slot.X) + float64(scene.Slot.Width)/2
	horizontal := clamp((scene.Target.X-headX)/math.Max(1, float64(scene.Content.Width)/2), -1, 1)
	vertical := clamp((scene.Target.Y-float64(scene.Content.Y))/math.Max(1, float64(scene.Content.Height)), 0, 1)
	pose.Yaw = yawRange * horizontal
	pose.Pitch = pitchBase + pitchRange*vertical
	return pose
}

// setBackground records the matching background for quantization.
func (m *Model) setBackground(c color.Color) {
	m.bg = c
}

// setMoving selects continuous motion. Automatic blinks only run while moving.
func (m *Model) setMoving(moving bool) {
	m.moving = moving
}

// setVisible flips visibility. Showing starts a fresh baseline, snaps to the
// current target with zero velocity and needs no catch-up; hiding invalidates
// the pending token, clears the baseline and velocities.
func (m *Model) setVisible(visible bool, at time.Time) bool {
	if visible == m.visible {
		return false
	}
	m.visible = visible
	if visible {
		m.visibleAt = at
		m.manualAt = time.Time{}
		m.lastHandled = time.Time{}
		m.nextDeadline = at.Add(frameInterval)
		m.pose = m.target
		m.vel = poseVel{}
		m.render(at)
	} else {
		m.pending = false
		m.seq++
		m.visibleAt = time.Time{}
		m.manualAt = time.Time{}
		m.lastHandled = time.Time{}
		m.nextDeadline = time.Time{}
		m.vel = poseVel{}
	}
	return true
}

// holdPose snaps directly to a pose with no spring step, used by the held
// preview path. It renders the new appearance.
func (m *Model) holdPose(p Pose, at time.Time) bool {
	m.setTarget(p)
	m.pose = m.target
	m.vel = poseVel{}
	return m.render(at)
}

// startBlink begins one manual blink envelope on the shared clock.
func (m *Model) startBlink(at time.Time) {
	m.manualAt = at
}

// validateFrame accepts only the live token: same owner, a pending tick, the
// current sequence and a visible model. It clears pending only on success.
func (m *Model) validateFrame(msg FrameMsg) bool {
	if msg.owner != m || !m.pending || msg.seq != m.seq || !m.visible {
		return false
	}
	m.pending = false
	return true
}

// advancePose steps the springs by the bounded real elapsed time and renders.
// The first step after showing uses one frame interval; a zero or reversed
// delta skips the spring math entirely.
func (m *Model) advancePose(now time.Time) bool {
	dt := frameInterval
	if !m.lastHandled.IsZero() {
		dt = now.Sub(m.lastHandled)
		if dt < 0 {
			dt = 0
		} else if dt > maxFrameStep {
			dt = maxFrameStep
		}
	}
	if now.After(m.lastHandled) {
		m.lastHandled = now
	}
	if dt > 0 {
		m.advance(dt)
	}
	return m.render(now)
}

// advance moves every channel one bounded step toward its target. Yaw and
// pitch share one critically damped spring; bob is under-damped; eye and lift
// use critical easing.
func (m *Model) advance(dt time.Duration) {
	secs := dt.Seconds()
	angle := harmonica.NewSpring(secs, 18, 1)
	bob := harmonica.NewSpring(secs, 20, 0.65)
	ease := harmonica.NewSpring(secs, 24, 1)

	m.pose.Yaw, m.vel.yaw = springStep(angle, m.pose.Yaw, m.vel.yaw, m.target.Yaw, -35, 35, angleEps)
	m.pose.Pitch, m.vel.pitch = springStep(angle, m.pose.Pitch, m.vel.pitch, m.target.Pitch, 0, 20, angleEps)
	m.pose.Bob, m.vel.bob = springStep(bob, m.pose.Bob, m.vel.bob, m.target.Bob, -0.06, 0.06, valueEps)
	m.pose.EyeOpen, m.vel.eye = springStep(ease, m.pose.EyeOpen, m.vel.eye, m.target.EyeOpen, 0, 1, valueEps)
	m.pose.Lift, m.vel.lift = springStep(ease, m.pose.Lift, m.vel.lift, m.target.Lift, -0.035, 0.10, valueEps)
}

// springStep advances one channel, clamps it to its renderer bounds, zeros
// outward velocity at a bound and snaps settled error/velocity.
func springStep(s harmonica.Spring, pos, vel, target, lo, hi, eps float64) (float64, float64) {
	if math.Abs(pos-target) < eps && math.Abs(vel) < eps {
		return target, 0
	}
	pos, vel = s.Update(pos, vel, target)
	if pos < lo {
		pos = lo
		if vel < 0 {
			vel = 0
		}
	} else if pos > hi {
		pos = hi
		if vel > 0 {
			vel = 0
		}
	}
	if math.Abs(pos-target) < eps && math.Abs(vel) < eps {
		return target, 0
	}
	return pos, vel
}

// render raycasts the current pose with the blink envelope applied after base
// aperture smoothing, so lids cover pupils immediately. Hidden models do no
// work.
func (m *Model) render(at time.Time) bool {
	if !m.visible {
		return false
	}
	rendered := m.pose
	blink := m.blinkFactor(at)
	if flinch := m.reaction.flinchFactor(at); flinch > 0 {
		rendered.EyeOpen = lerp(rendered.EyeOpen, flinchMood.EyeOpen, flinch)
		rendered.Lift = lerp(rendered.Lift, flinchMood.Lift, flinch)
	} else {
		rendered.EyeOpen *= blink
	}
	return m.renderer.Render(rendered, m.bg)
}

// lerp blends a toward b by t in 0..1, exact at the endpoints.
func lerp(a, b, t float64) float64 {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	return a + (b-a)*t
}

// blinkFactor combines any manual blink with the automatic 4 s blink. Automatic
// blinks only run while the model is continuously moving.
func (m *Model) blinkFactor(at time.Time) float64 {
	factor := 1.0
	if !m.manualAt.IsZero() {
		if d := at.Sub(m.manualAt); d >= blinkDuration {
			m.manualAt = time.Time{}
		} else if d >= 0 {
			factor = math.Min(factor, blinkEnvelope(d))
		}
	}
	if m.moving && m.visible && !m.visibleAt.IsZero() {
		if elapsed := at.Sub(m.visibleAt); elapsed >= blinkPeriod {
			if phase := (elapsed - blinkPeriod) % blinkPeriod; phase < blinkDuration {
				factor = math.Min(factor, blinkEnvelope(phase))
			}
		}
	}
	return factor
}

// manualBlinkActive reports whether a manual blink still needs the clock.
func (m *Model) manualBlinkActive(at time.Time) bool {
	return !m.manualAt.IsZero() && at.Sub(m.manualAt) < blinkDuration
}

// blinkEnvelope is the 1 -> 0 -> 1 aperture multiplier over one blink.
func blinkEnvelope(d time.Duration) float64 {
	if d <= 0 || d >= blinkDuration {
		return 1
	}
	return math.Abs(1 - 2*float64(d)/float64(blinkDuration))
}

// armNext schedules exactly one successor when visible and either moving or
// mid-blink. The deadline advances by whole intervals until strictly in the
// future, so stalls skip slots instead of replaying them.
func (m *Model) armNext(now time.Time) tea.Cmd {
	if !m.visible || m.pending {
		return nil
	}
	if !m.moving && !m.manualBlinkActive(now) {
		return nil
	}
	if m.nextDeadline.IsZero() {
		m.nextDeadline = now.Add(frameInterval)
	} else if !m.nextDeadline.After(now) {
		steps := now.Sub(m.nextDeadline)/frameInterval + 1
		m.nextDeadline = m.nextDeadline.Add(steps * frameInterval)
	}

	m.seq++
	seq := m.seq
	owner := m
	delay := m.nextDeadline.Sub(now)
	if delay < 0 {
		delay = 0
	}
	m.pending = true
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return FrameMsg{owner: owner, seq: seq, at: t}
	})
}
