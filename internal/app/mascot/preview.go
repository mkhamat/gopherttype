package mascot

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// The interactive preview reserves a fixed 32x14 block: the unchanged 32x12 art
// plus two status/help rows outside it. Below that threshold it shows a resize
// hint instead of cropping, scaling or wrapping the art.
const (
	previewBlockWidth  = Width
	previewBlockHeight = Height + 2

	// helpLine is at most previewBlockWidth display cells wide so the block has
	// a clean right edge.
	helpLine = "←→↑↓ m mood a anim b blink r q"
)

// Mood is one named expression preset: eye openness and smile-corner lift.
// These are the rig's vocabulary values shared by the --mood flag and the
// interactive mood cycle.
type Mood struct {
	EyeOpen float64
	Lift    float64
}

// moodPresets is the developer expression cycle in order:
// calm -> proud -> worried -> sleepy -> flinch.
var moodPresets = []struct {
	name string
	Mood
}{
	{"calm", Mood{0.95, 0.06}},
	{"proud", Mood{1.00, 0.10}},
	{"worried", Mood{0.80, -0.035}},
	{"sleepy", Mood{0.45, 0}},
	{"flinch", Mood{0.08, 0}},
}

// MoodByName returns the named expression preset and whether it exists. Names
// are calm, proud, worried, sleepy and flinch.
func MoodByName(name string) (Mood, bool) {
	for _, m := range moodPresets {
		if m.name == name {
			return m.Mood, true
		}
	}
	return Mood{}, false
}

// PreviewSettings configures the interactive preview. A nil Background means
// "no explicit override": the model starts from the renderer's dark fallback
// and asks the terminal for its actual background once. Animate selects the
// procedural turn/blink demo over a held inspectable pose.
type PreviewSettings struct {
	Pose       Pose
	Background color.Color
	Animate    bool
}

// NewPreview returns a Bubble Tea model for the interactive preview. It owns
// one Model (and thus one Renderer) and composes its cached portrait into a
// centered block. Held mode schedules no clock; animated mode drives the
// model's shared one-shot chain.
func NewPreview(settings PreviewSettings) tea.Model {
	now := time.Now()
	p := &preview{
		model:    New(),
		override: settings.Background != nil,
		held:     sanitizePose(settings.Pose),
		animate:  settings.Animate,
		// A sane fallback until the first WindowSizeMsg reports the real size.
		width:  80,
		height: 24,
		shown:  true,
	}
	if settings.Background != nil {
		p.model.setBackground(settings.Background)
	}
	p.mood = moodIndexFor(p.held)
	p.model.setTarget(p.held)
	p.model.setMoving(settings.Animate)
	p.model.setVisible(true, now)
	if settings.Animate {
		p.animAt = now
	}
	p.view = p.compose()
	p.status = p.statusLine()
	return p
}

// preview is the private preview adapter. It owns one Model (renderer, pose,
// springs and clock), the direct held pose, the animation baseline and the
// cached composed view.
type preview struct {
	model    *Model
	override bool // explicit --background; suppresses the background request
	animate  bool
	held     Pose

	width, height int
	mood          int // index into moodPresets, or -1 for a custom pose
	animAt        time.Time
	shown         bool
	view          string
	status        string
}

// Init requests the terminal background once unless the user pinned one; the
// first WindowSizeMsg arms the clock in animated mode.
func (p *preview) Init() tea.Cmd {
	if p.override {
		return nil
	}
	return tea.RequestBackgroundColor
}

// Update re-renders for held changes and advances the shared clock for frames.
func (p *preview) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case FrameMsg:
		return p, p.onFrame(msg)
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		return p, p.applyLayout(time.Now())
	case tea.BackgroundColorMsg:
		if p.override {
			return p, nil
		}
		p.model.setBackground(msg.Color)
		p.model.render(time.Now())
		p.view = p.compose()
		return p, nil
	case tea.KeyPressMsg:
		return p, p.handleKey(msg)
	}
	return p, nil
}

// View wraps the cached composition in an alternate-screen view.
func (p *preview) View() tea.View {
	v := tea.NewView(p.view)
	v.AltScreen = true
	return v
}

// onFrame validates one clock tick, supplies the procedural target when
// animating, advances the shared step and arms the successor.
func (p *preview) onFrame(msg FrameMsg) tea.Cmd {
	if !p.model.validateFrame(msg) {
		return nil
	}
	now := time.Now()
	if p.animate {
		p.model.setTarget(p.formulaTarget(now))
	}
	p.refresh(p.model.advancePose(now))
	return p.model.armNext(now)
}

// applyLayout matches visibility and animation to the current window size. An
// undersized window hides the model and invalidates the clock; a fitting window
// resumes exactly the requested mode with a fresh baseline.
func (p *preview) applyLayout(now time.Time) tea.Cmd {
	fits := p.width >= previewBlockWidth && p.height >= previewBlockHeight
	if !fits {
		if p.shown {
			p.shown = false
			p.model.setVisible(false, now)
			p.animAt = time.Time{}
		}
		p.view = p.compose()
		return nil
	}

	if !p.shown {
		p.shown = true
		p.model.setTarget(p.held)
		p.model.setVisible(true, now)
	}

	if p.animate {
		p.model.setMoving(true)
		if p.animAt.IsZero() {
			p.animAt = now
			p.model.visibleAt = now
		}
		p.view = p.compose()
		return p.model.armNext(now)
	}

	p.model.setMoving(false)
	p.model.holdPose(p.held, now)
	p.view = p.compose()
	p.status = p.statusLine()
	return nil
}

// handleKey applies one developer control. Arrows stop continuous motion and
// inspect a held pose; `a` toggles motion; `b` starts one manual blink; `m`
// cycles the expression preserving the animation mode; `r` resets and stops.
func (p *preview) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	now := time.Now()
	switch msg.String() {
	case "up":
		p.detach()
		p.held.Pitch = clamp(p.held.Pitch-2, 0, 20)
		p.hold(now)
	case "down":
		p.detach()
		p.held.Pitch = clamp(p.held.Pitch+2, 0, 20)
		p.hold(now)
	case "left":
		p.detach()
		p.held.Yaw = clamp(p.held.Yaw-5, -35, 35)
		p.hold(now)
	case "right":
		p.detach()
		p.held.Yaw = clamp(p.held.Yaw+5, -35, 35)
		p.hold(now)
	case "m":
		p.cycleMood()
		if !p.animate {
			p.hold(now)
		}
	case "a":
		if p.animate {
			p.detach()
			p.model.setMoving(false)
			p.hold(now)
			return nil
		}
		p.animate = true
		p.animAt = now
		p.model.visibleAt = now
		p.model.setMoving(true)
		return p.model.armNext(now)
	case "b":
		p.model.startBlink(now)
		return p.model.armNext(now)
	case "r":
		p.animate = false
		p.held = NeutralPose()
		p.mood = moodIndexFor(p.held)
		p.hold(now)
	case "q", "esc", "ctrl+c":
		p.model.setVisible(false, now)
		return tea.Quit
	default:
		return nil
	}
	return nil
}

// hold snaps to the direct held pose with no spring step and recomposes.
func (p *preview) hold(now time.Time) {
	p.model.setMoving(false)
	p.model.holdPose(p.held, now)
	p.view = p.compose()
	p.status = p.statusLine()
}

// detach stops continuous motion and captures the current rendered pose as the
// inspectable held pose, so inspecting mid-turn does not jump.
func (p *preview) detach() {
	if p.animate {
		p.animate = false
		p.held = p.model.pose
	}
}

// formulaTarget evaluates the procedural demo pose at elapsed animation time.
// The selected mood sets eye/lift; only proud bobs. These targets resemble the
// JS demo but are controller behavior, not renderer parity.
func (p *preview) formulaTarget(now time.Time) Pose {
	t := now.Sub(p.animAt).Seconds()
	target := Pose{
		Yaw:     30 * math.Sin(t*math.Pi/3),
		Pitch:   12 + 6*math.Sin(t*math.Pi/6),
		EyeOpen: p.currentMood().EyeOpen,
		Lift:    p.currentMood().Lift,
	}
	if p.mood >= 0 && moodPresets[p.mood].name == "proud" {
		target.Bob = 0.025 * math.Sin(4*math.Pi*t)
	}
	return target
}

// currentMood returns the active expression preset, defaulting to calm for a
// custom held pose.
func (p *preview) currentMood() Mood {
	if p.mood >= 0 {
		return moodPresets[p.mood].Mood
	}
	return moodPresets[0].Mood
}

// refresh recomposes only when the art or the status text changed.
func (p *preview) refresh(artChanged bool) {
	status := p.statusLine()
	if artChanged || status != p.status {
		p.status = status
		p.view = p.compose()
	}
}

// cycleMood advances the expression preset, keeping the current yaw/pitch and
// bob. A custom pose starts the cycle at calm.
func (p *preview) cycleMood() {
	next := 0
	if p.mood >= 0 {
		next = (p.mood + 1) % len(moodPresets)
	}
	p.mood = next
	p.held.EyeOpen = moodPresets[next].EyeOpen
	p.held.Lift = moodPresets[next].Lift
}

// compose centers the fixed preview block, or a resize hint when the window is
// too small. It never crops, scales or wraps the art.
func (p *preview) compose() string {
	if p.width < previewBlockWidth || p.height < previewBlockHeight {
		return p.resizeView()
	}
	art := strings.Split(p.model.View(), "\n")
	rows := make([]string, 0, previewBlockHeight)
	rows = append(rows, art...)
	rows = append(rows, padToBlock(p.statusLine()), padToBlock(helpLine))
	return centerBlock(rows, p.width, p.height)
}

// resizeView is a fitting hint shown when the window cannot hold 32x14.
func (p *preview) resizeView() string {
	lines := []string{"terminal too small", fmt.Sprintf("need %dx%d", previewBlockWidth, previewBlockHeight)}

	width, height := p.width, p.height
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "")
	}

	top := (height - len(lines)) / 2
	if top < 0 {
		top = 0
	}
	var b strings.Builder
	for i := 0; i < top; i++ {
		b.WriteByte('\n')
	}
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if pad := (width - ansi.StringWidth(line)) / 2; pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		b.WriteString(line)
	}
	return b.String()
}

// statusLine reports the held pose with the renderer's clamped values and the
// current mode.
func (p *preview) statusLine() string {
	name := "custom"
	if p.mood >= 0 {
		name = moodPresets[p.mood].name
	}
	mode := "held"
	if p.animate {
		mode = "anim"
	}
	return fmt.Sprintf("yaw %+.0f  pitch %+.0f  %s %s", p.model.pose.Yaw, p.model.pose.Pitch, name, mode)
}

// moodIndexFor returns the preset matching a pose's eye/lift, or -1 for a
// custom expression.
func moodIndexFor(pose Pose) int {
	for i, m := range moodPresets {
		if pose.EyeOpen == m.EyeOpen && pose.Lift == m.Lift {
			return i
		}
	}
	return -1
}

// padToBlock truncates then right-pads a status row to the block width.
func padToBlock(s string) string {
	s = ansi.Truncate(s, previewBlockWidth, "")
	if w := ansi.StringWidth(s); w < previewBlockWidth {
		s += strings.Repeat(" ", previewBlockWidth-w)
	}
	return s
}

// centerBlock centers fixed-width rows within the window without re-wrapping.
func centerBlock(rows []string, width, height int) string {
	left := (width - previewBlockWidth) / 2
	top := (height - len(rows)) / 2

	var b strings.Builder
	for i := 0; i < top; i++ {
		b.WriteByte('\n')
	}
	pad := strings.Repeat(" ", left)
	for i, row := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(pad)
		b.WriteString(row)
	}
	return b.String()
}
