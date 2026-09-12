package mascot

import (
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func neutralSettings() PreviewSettings {
	return PreviewSettings{Pose: NeutralPose()}
}

func newTestPreview(t *testing.T, s PreviewSettings) *preview {
	t.Helper()
	m := NewPreview(s)
	p, ok := m.(*preview)
	if !ok {
		t.Fatalf("NewPreview returned %T, want *preview", m)
	}
	return p
}

func key(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func textKey(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: s, Code: []rune(s)[0]})
}

func ctrlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: tea.ModCtrl})
}

func press(t *testing.T, p *preview, k tea.KeyPressMsg) tea.Cmd {
	t.Helper()
	_, cmd := p.Update(k)
	return cmd
}

// TestPreviewArrowControls pins yaw ±5°, pitch ±2° and their renderer bounds.
func TestPreviewArrowControls(t *testing.T) {
	p := newTestPreview(t, neutralSettings())

	press(t, p, key(tea.KeyRight))
	if p.model.pose.Yaw != 5 {
		t.Errorf("right: yaw = %g, want 5", p.model.pose.Yaw)
	}
	press(t, p, key(tea.KeyLeft))
	press(t, p, key(tea.KeyLeft))
	if p.model.pose.Yaw != -5 {
		t.Errorf("left twice: yaw = %g, want -5", p.model.pose.Yaw)
	}

	press(t, p, key(tea.KeyDown))
	if p.model.pose.Pitch != 2 {
		t.Errorf("down: pitch = %g, want 2", p.model.pose.Pitch)
	}
	press(t, p, key(tea.KeyUp))
	if p.model.pose.Pitch != 0 {
		t.Errorf("up: pitch = %g, want 0", p.model.pose.Pitch)
	}
	press(t, p, key(tea.KeyUp))
	if p.model.pose.Pitch != 0 {
		t.Errorf("pitch must clamp at 0, got %g", p.model.pose.Pitch)
	}
}

// TestPreviewControlBounds holds each arrow past its limit and checks the
// renderer's clamp bounds.
func TestPreviewControlBounds(t *testing.T) {
	p := newTestPreview(t, neutralSettings())
	for i := 0; i < 20; i++ {
		press(t, p, key(tea.KeyRight))
	}
	if p.model.pose.Yaw != 35 {
		t.Errorf("yaw max = %g, want 35", p.model.pose.Yaw)
	}
	for i := 0; i < 20; i++ {
		press(t, p, key(tea.KeyLeft))
	}
	if p.model.pose.Yaw != -35 {
		t.Errorf("yaw min = %g, want -35", p.model.pose.Yaw)
	}
	for i := 0; i < 30; i++ {
		press(t, p, key(tea.KeyDown))
	}
	if p.model.pose.Pitch != 20 {
		t.Errorf("pitch max = %g, want 20", p.model.pose.Pitch)
	}
}

// TestPreviewMoodCycle checks the calm → proud → worried → sleepy → flinch
// order wraps back to calm while preserving yaw/pitch.
func TestPreviewMoodCycle(t *testing.T) {
	p := newTestPreview(t, neutralSettings())
	press(t, p, key(tea.KeyRight)) // yaw 5 must survive mood changes
	want := []string{"proud", "worried", "sleepy", "flinch", "calm"}
	for _, name := range want {
		press(t, p, textKey("m"))
		if got := moodPresets[p.mood].name; got != name {
			t.Fatalf("mood cycle = %q, want %q", got, name)
		}
		if p.model.pose.EyeOpen != moodPresets[p.mood].EyeOpen || p.model.pose.Lift != moodPresets[p.mood].Lift {
			t.Errorf("mood %q did not set eye/lift", name)
		}
		if p.model.pose.Yaw != 5 {
			t.Errorf("mood %q changed yaw to %g", name, p.model.pose.Yaw)
		}
	}
}

// TestPreviewResetRestoresNeutral checks `r` restores the current neutral pose
// (pitch 0), including the mood index, and stops motion.
func TestPreviewResetRestoresNeutral(t *testing.T) {
	p := newTestPreview(t, neutralSettings())
	press(t, p, key(tea.KeyDown))
	press(t, p, key(tea.KeyRight))
	press(t, p, textKey("m")) // proud

	press(t, p, textKey("r"))
	if p.model.pose != NeutralPose() {
		t.Errorf("reset pose = %+v, want neutral %+v", p.model.pose, NeutralPose())
	}
	if p.model.pose.Pitch != 0 {
		t.Errorf("reset must use neutral pitch 0, got %g", p.model.pose.Pitch)
	}
	if p.mood != 0 {
		t.Errorf("reset mood index = %d, want calm (0)", p.mood)
	}
	if p.animate {
		t.Error("reset must stop continuous motion")
	}
}

// TestPreviewResizeThresholds exercises the exact 32x14 boundary: widths
// 31/32/33 and heights 13/14/15.
func TestPreviewResizeThresholds(t *testing.T) {
	for _, tc := range []struct {
		w, h    int
		visible bool
	}{
		{31, 14, false}, {32, 14, true}, {33, 14, true},
		{32, 13, false}, {32, 14, true}, {32, 15, true},
	} {
		p := newTestPreview(t, neutralSettings())
		p.Update(tea.WindowSizeMsg{Width: tc.w, Height: tc.h})
		hidden := strings.Contains(p.view, "terminal too small")
		if tc.visible == hidden {
			t.Errorf("%dx%d: hidden=%v, want visible=%v", tc.w, tc.h, hidden, tc.visible)
		}
	}
}

// TestPreviewVisibleLayout checks the visible block is 14 rows: the fixed 32x12
// art plus two 32-cell status/help rows outside it.
func TestPreviewVisibleLayout(t *testing.T) {
	p := newTestPreview(t, neutralSettings())
	p.Update(tea.WindowSizeMsg{Width: 32, Height: 14})
	lines := strings.Split(p.view, "\n")
	if len(lines) != previewBlockHeight {
		t.Fatalf("got %d rows, want %d", len(lines), previewBlockHeight)
	}
	for y, line := range lines {
		if w := ansi.StringWidth(ansi.Strip(line)); w != previewBlockWidth {
			t.Errorf("row %d width %d, want %d", y, w, previewBlockWidth)
		}
	}
	if !strings.Contains(ansi.Strip(lines[Height]), "yaw") {
		t.Errorf("status row missing pose: %q", ansi.Strip(lines[Height]))
	}
	if !strings.Contains(ansi.Strip(lines[Height+1]), "mood") {
		t.Errorf("help row missing controls: %q", ansi.Strip(lines[Height+1]))
	}
}

// TestPreviewHelpLineFits keeps the help text inside the 32-cell block.
func TestPreviewHelpLineFits(t *testing.T) {
	if w := ansi.StringWidth(helpLine); w > previewBlockWidth {
		t.Errorf("help line width %d exceeds block width %d", w, previewBlockWidth)
	}
}

// TestPreviewViewStable proves View never renders, mutates or reallocates.
func TestPreviewViewStable(t *testing.T) {
	p := newTestPreview(t, neutralSettings())
	first := p.view
	for i := 0; i < 3; i++ {
		if got := p.View().Content; got != first {
			t.Fatal("View must return the cached string unchanged")
		}
	}
	if cmd := press(t, p, textKey("z")); cmd != nil {
		t.Error("unrelated key should not schedule a command")
	}
	if p.view != first {
		t.Error("unrelated key must not change the held view")
	}
}

// TestPreviewNoFrameCommands proves held mode schedules nothing: no keys, no
// resize and no background change arm a frame command.
func TestPreviewNoFrameCommands(t *testing.T) {
	p := newTestPreview(t, neutralSettings())
	for _, k := range []tea.KeyPressMsg{
		key(tea.KeyRight), key(tea.KeyLeft), key(tea.KeyUp), key(tea.KeyDown),
		textKey("m"), textKey("r"), textKey("z"),
	} {
		if cmd := press(t, p, k); cmd != nil {
			t.Errorf("key %q scheduled a command in held mode", k.String())
		}
	}
	if _, cmd := p.Update(tea.WindowSizeMsg{Width: 40, Height: 20}); cmd != nil {
		t.Error("resize must not schedule a frame in held mode")
	}
	if _, cmd := p.Update(tea.BackgroundColorMsg{Color: color.Black}); cmd != nil {
		t.Error("background change must not schedule a frame")
	}
}

// TestPreviewQuitKeys checks q, Esc and Ctrl+C all quit and invalidate motion.
func TestPreviewQuitKeys(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{textKey("q"), key(tea.KeyEscape), ctrlKey('c')} {
		p := newTestPreview(t, neutralSettings())
		if press(t, p, k) == nil {
			t.Errorf("key %q should quit", k.String())
		}
		if p.model.visible {
			t.Errorf("key %q must hide before quitting", k.String())
		}
	}
}

// TestPreviewBackgroundRequest is initialization-only: without an override Init
// asks once; an explicit override suppresses the request.
func TestPreviewBackgroundRequest(t *testing.T) {
	if NewPreview(neutralSettings()).Init() == nil {
		t.Error("no override: Init should request the terminal background once")
	}
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	if NewPreview(PreviewSettings{Pose: NeutralPose(), Background: white}).Init() != nil {
		t.Error("explicit override: Init must not request the background")
	}
}

// TestPreviewBackgroundUpdate re-renders on the real color when there is no
// override and ignores it when one was pinned.
func TestPreviewBackgroundUpdate(t *testing.T) {
	pose := Pose{Yaw: 35, Pitch: 14, EyeOpen: 0.95, Lift: 0.06}
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}

	p := newTestPreview(t, PreviewSettings{Pose: pose})
	before := p.view
	p.Update(tea.BackgroundColorMsg{Color: white})
	if p.view == before {
		t.Error("background change should recompose when cells change")
	}

	dark := color.RGBA{R: 0x16, G: 0x1b, B: 0x22, A: 0xff}
	pinned := newTestPreview(t, PreviewSettings{Pose: pose, Background: dark})
	held := pinned.view
	pinned.Update(tea.BackgroundColorMsg{Color: white})
	if pinned.view != held {
		t.Error("explicit background override must ignore the terminal color")
	}
}

// TestPreviewUsesRenderer confirms the preview renders through the shared
// renderer, not a second path.
func TestPreviewUsesRenderer(t *testing.T) {
	pose := Pose{Yaw: 10, Pitch: 5, EyeOpen: 0.80, Lift: -0.035}
	p := newTestPreview(t, PreviewSettings{Pose: pose})
	r := NewRenderer()
	r.Render(sanitizePose(pose), nil)
	if p.model.renderer.View() != r.View() {
		t.Error("preview art must match the shared renderer output")
	}
}

// TestPreviewAnimateArms checks animated mode arms exactly one clock on the
// initial size message and does not arm twice while pending.
func TestPreviewAnimateArms(t *testing.T) {
	p := newTestPreview(t, PreviewSettings{Pose: NeutralPose(), Animate: true})
	_, cmd := p.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd == nil {
		t.Fatal("animated preview should arm a frame clock")
	}
	if !p.model.pending {
		t.Error("arming should mark a pending tick")
	}
	if _, cmd := p.Update(tea.WindowSizeMsg{Width: 80, Height: 24}); cmd != nil {
		t.Error("a second size message must not arm another chain while pending")
	}
}

// TestPreviewToggleMotion checks `a` toggles continuous motion and retains the
// current pose as the inspectable held pose.
func TestPreviewToggleMotion(t *testing.T) {
	p := newTestPreview(t, PreviewSettings{Pose: Pose{Yaw: 20, Pitch: 6, EyeOpen: 0.95, Lift: 0.06}})
	if cmd := press(t, p, textKey("a")); cmd == nil {
		t.Error("enabling motion should arm the clock")
	}
	press(t, p, textKey("a"))
	if p.animate {
		t.Error("second `a` should stop motion")
	}
	if p.held.Yaw != p.model.pose.Yaw {
		t.Errorf("stopping should retain the inspectable pose, held yaw %g, pose %g", p.held.Yaw, p.model.pose.Yaw)
	}
}

// TestPreviewManualBlinkArmsHeld checks `b` arms a one-shot clock while held.
func TestPreviewManualBlinkArmsHeld(t *testing.T) {
	p := newTestPreview(t, neutralSettings())
	if cmd := press(t, p, textKey("b")); cmd == nil {
		t.Error("manual blink should arm a one-shot clock")
	}
	if p.model.manualAt.IsZero() {
		t.Error("manual blink should record a start time")
	}
}

// TestPreviewFormulaTarget pins the procedural demo formulas and their signs.
func TestPreviewFormulaTarget(t *testing.T) {
	p := newTestPreview(t, PreviewSettings{Pose: NeutralPose(), Animate: true})
	base := time.Now()
	p.animAt = base

	start := p.formulaTarget(base)
	if start.Yaw != 0 || start.Pitch != 12 {
		t.Errorf("t=0 target = (%g,%g), want (0,12)", start.Yaw, start.Pitch)
	}
	quarter := p.formulaTarget(base.Add(1500 * time.Millisecond))
	if quarter.Yaw <= 0 || quarter.Yaw > 30 {
		t.Errorf("t=1.5s yaw = %g, want a positive turn", quarter.Yaw)
	}
	if quarter.Pitch < 6 || quarter.Pitch > 20 {
		t.Errorf("t=1.5s pitch = %g, want within 6..20", quarter.Pitch)
	}
	// Only proud bobs.
	p.cycleMood() // calm -> proud
	p.mood = 1
	if bob := p.formulaTarget(base.Add(125 * time.Millisecond)).Bob; bob == 0 {
		t.Error("proud should bob")
	}
	p.mood = 0
	if bob := p.formulaTarget(base.Add(125 * time.Millisecond)).Bob; bob != 0 {
		t.Errorf("calm bob = %g, want 0", bob)
	}
}

// TestPreviewHeldKeysDoNotStep checks arrow/mood keys set the direct pose with
// no spring step and no frame command.
func TestPreviewHeldKeysDoNotStep(t *testing.T) {
	p := newTestPreview(t, neutralSettings())
	if cmd := press(t, p, key(tea.KeyRight)); cmd != nil {
		t.Error("held arrow must not schedule a frame")
	}
	if p.model.pose.Yaw != 5 {
		t.Errorf("held arrow should set the direct pose, got %g", p.model.pose.Yaw)
	}
	if p.model.vel != (poseVel{}) {
		t.Error("held arrow must not leave velocity")
	}
}

// TestPreviewDetachCapturesCurrentPose checks that inspecting mid-animation
// keeps the animated pose instead of jumping back to a stale held pose.
func TestPreviewDetachCapturesCurrentPose(t *testing.T) {
	p := newTestPreview(t, PreviewSettings{Pose: NeutralPose(), Animate: true})
	_, cmd := p.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	for i := 0; i < 8 && cmd != nil; i++ {
		msg := cmd()
		_, cmd = p.Update(msg)
	}
	current := p.model.pose
	press(t, p, key(tea.KeyUp))
	if p.animate {
		t.Fatal("arrow should stop continuous motion")
	}
	if want := clamp(current.Pitch-2, 0, 20); p.held.Pitch != want {
		t.Errorf("held pitch %g, want %g derived from current pose", p.held.Pitch, want)
	}
}
