package mascot

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// The interactive preview reserves a fixed 32x14 block: the unchanged 32x12 art
// plus two status/help rows outside it. Below that threshold it shows a resize
// hint instead of cropping, scaling or wrapping the art.
const (
	previewBlockWidth  = Width
	previewBlockHeight = Height + 2

	// helpLine is exactly previewBlockWidth display cells wide so the block has
	// a clean right edge.
	helpLine = "←→ yaw ↑↓ pitch m mood r reset q"
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
	{"sleepy", Mood{0.20, 0}},
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

// PreviewSettings configures the interactive still preview. A nil Background
// means "no explicit override": the model starts from the renderer's dark
// fallback and asks the terminal for its actual background once. Ticket 04 adds
// Animate when it has a consumer; do not add it here.
type PreviewSettings struct {
	Pose       Pose
	Background color.Color
}

// NewPreview returns a Bubble Tea model for the interactive held-pose preview.
// It renders once at construction and refreshes only on key, size and
// background changes; it never schedules a clock.
func NewPreview(settings PreviewSettings) tea.Model {
	p := &preview{
		renderer: NewRenderer(),
		pose:     sanitizePose(settings.Pose),
		override: settings.Background != nil,
		// A sane fallback until the first WindowSizeMsg reports the real size.
		width:  80,
		height: 24,
	}
	if settings.Background != nil {
		p.bg = settings.Background
	}
	p.mood = moodIndexFor(p.pose)
	p.render()
	return p
}

// preview is the private still-preview adapter. It owns the current pose, the
// matching background, one renderer and the cached composed view. View never
// renders or reads time.
type preview struct {
	renderer *Renderer
	pose     Pose
	bg       color.Color
	override bool // explicit --background; suppresses the background request

	width, height int
	mood          int // index into moodPresets, or -1 for a custom pose
	view          string
}

// Init requests the terminal background once unless the user pinned one. Held
// mode schedules no frames.
func (p *preview) Init() tea.Cmd {
	if p.override {
		return nil
	}
	return tea.RequestBackgroundColor
}

// Update re-renders only for the messages that can change the held frame.
func (p *preview) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width == p.width && msg.Height == p.height {
			return p, nil
		}
		p.width, p.height = msg.Width, msg.Height
		p.view = p.compose()
	case tea.BackgroundColorMsg:
		if p.override {
			return p, nil
		}
		p.bg = msg.Color
		p.render()
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

// handleKey applies one developer control. Every control shares this switch and
// returns no frame command; only quit returns a command.
func (p *preview) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "up":
		p.pose.Pitch = clamp(p.pose.Pitch-2, 0, 20)
	case "down":
		p.pose.Pitch = clamp(p.pose.Pitch+2, 0, 20)
	case "left":
		p.pose.Yaw = clamp(p.pose.Yaw-5, -35, 35)
	case "right":
		p.pose.Yaw = clamp(p.pose.Yaw+5, -35, 35)
	case "m":
		p.cycleMood()
	case "r":
		p.pose = NeutralPose()
		p.mood = moodIndexFor(p.pose)
	case "q", "esc", "ctrl+c":
		return tea.Quit
	default:
		return nil
	}
	p.render()
	return nil
}

// cycleMood advances the expression preset, keeping the current yaw/pitch and
// bob. A custom pose starts the cycle at calm.
func (p *preview) cycleMood() {
	next := 0
	if p.mood >= 0 {
		next = (p.mood + 1) % len(moodPresets)
	}
	p.mood = next
	p.pose.EyeOpen = moodPresets[next].EyeOpen
	p.pose.Lift = moodPresets[next].Lift
}

// render refreshes the art and recomposes the cached view.
func (p *preview) render() {
	p.renderer.Render(p.pose, p.bg)
	p.view = p.compose()
}

// compose centers the fixed preview block, or a resize hint when the window is
// too small. It never crops, scales or wraps the art.
func (p *preview) compose() string {
	if p.width < previewBlockWidth || p.height < previewBlockHeight {
		return p.resizeView()
	}
	art := strings.Split(p.renderer.View(), "\n")
	rows := make([]string, 0, previewBlockHeight)
	rows = append(rows, art...)
	rows = append(rows, padToBlock(p.statusLine()), padToBlock(helpLine))
	return centerBlock(rows, p.width, p.height)
}

// resizeView is a fitting hint shown when the window cannot hold 32x14.
func (p *preview) resizeView() string {
	lines := []string{"terminal too small", fmt.Sprintf("need %dx%d", previewBlockWidth, previewBlockHeight)}

	width, height := p.width, p.height
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
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

// statusLine reports the held pose with the renderer's clamped values.
func (p *preview) statusLine() string {
	name := "custom"
	if p.mood >= 0 && p.mood < len(moodPresets) {
		name = moodPresets[p.mood].name
	}
	return fmt.Sprintf("yaw %+.0f  pitch %+.0f  %s", p.pose.Yaw, p.pose.Pitch, name)
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
	if left < 0 {
		left = 0
	}
	top := (height - len(rows)) / 2
	if top < 0 {
		top = 0
	}

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
