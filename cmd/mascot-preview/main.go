package main

import (
	"errors"
	"flag"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"gopherttype/internal/app/mascot"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// interactiveTTY reports whether stdout and stdin are real terminals. It is a
// variable so tests can reach the interactive branch without a pty.
var interactiveTTY = func(stdout io.Writer) bool {
	f, ok := stdout.(*os.File)
	return ok && term.IsTerminal(f.Fd()) && term.IsTerminal(os.Stdin.Fd())
}

// runPreviewProgram launches the interactive Bubble Tea program. It is a
// variable so tests can observe the interactive branch without a terminal.
var runPreviewProgram = func(model tea.Model, stdout io.Writer) error {
	program := tea.NewProgram(model, tea.WithInput(os.Stdin), tea.WithOutput(stdout), tea.WithFPS(120))
	_, err := program.Run()
	return err
}

// run parses the preview flags. The default invocation runs the interactive
// held-pose preview (a TTY is required); --plain prints one ANSI frame to
// stdout and needs no terminal. It returns a process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mascot-preview", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, "Usage: mascot-preview [--plain] [--animate] [--yaw D] [--pitch D] [--mood NAME] [--eye V] [--lift V] [--bob V] [--background #RRGGBB]\n\n")
		fs.PrintDefaults()
	}

	plain := fs.Bool("plain", false, "print one ANSI frame to stdout instead of running interactively")
	animate := fs.Bool("animate", false, "continuously turn the head and blink at a 120 Hz target")
	yaw := fs.Float64("yaw", 0, "head yaw in degrees, clamped to -35..35")
	pitch := fs.Float64("pitch", 0, "head pitch in degrees, clamped to 0..20")
	moodName := fs.String("mood", "", "expression mood: calm, proud, worried, sleepy, flinch")
	eye := fs.Float64("eye", 0, "eye openness, clamped to 0..1")
	lift := fs.Float64("lift", 0, "smile-corner lift, clamped to -0.035..0.10")
	bob := fs.Float64("bob", 0, "head bob, clamped to -0.06..0.06")
	background := fs.String("background", "", "matching background color as #RRGGBB")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		// The flag package already reported the error and usage to stderr.
		return 2
	}

	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	for _, f := range []struct {
		name  string
		value float64
	}{
		{"yaw", *yaw}, {"pitch", *pitch}, {"eye", *eye}, {"lift", *lift}, {"bob", *bob},
	} {
		if set[f.name] && !isFinite(f.value) {
			fmt.Fprintf(stderr, "-%s must be a finite number\n", f.name)
			return 1
		}
	}

	// Start from neutral, apply the mood preset, then let explicitly supplied
	// expression flags win regardless of argument order.
	pose := mascot.NeutralPose()
	if *moodName != "" {
		m, ok := mascot.MoodByName(*moodName)
		if !ok {
			fmt.Fprintf(stderr, "unknown mood %q: use calm, proud, worried, sleepy, or flinch\n", *moodName)
			return 1
		}
		pose.EyeOpen = m.EyeOpen
		pose.Lift = m.Lift
	}
	if set["yaw"] {
		pose.Yaw = *yaw
	}
	if set["pitch"] {
		pose.Pitch = *pitch
	}
	if set["eye"] {
		pose.EyeOpen = *eye
	}
	if set["lift"] {
		pose.Lift = *lift
	}
	if set["bob"] {
		pose.Bob = *bob
	}

	var bg color.Color
	if *background != "" {
		c, err := parseHexColor(*background)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		bg = c
	}

	// --plain stays a one-shot: render and return before Bubble Tea starts.
	if *plain {
		renderer := mascot.NewRenderer()
		renderer.Render(pose, bg)
		fmt.Fprintln(stdout, renderer.View())
		return 0
	}

	if !interactiveTTY(stdout) {
		fmt.Fprintln(stderr, "mascot-preview: interactive mode needs a terminal; use --plain to print one frame")
		return 1
	}

	model := mascot.NewPreview(mascot.PreviewSettings{Pose: pose, Background: bg, Animate: *animate})
	if err := runPreviewProgram(model, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// parseHexColor accepts #RRGGBB (upper or lower case).
func parseHexColor(s string) (color.RGBA, error) {
	if len(s) != 7 || s[0] != '#' {
		return color.RGBA{}, fmt.Errorf("background must be #RRGGBB, got %q", s)
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("background must be #RRGGBB, got %q", s)
	}
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}, nil
}
