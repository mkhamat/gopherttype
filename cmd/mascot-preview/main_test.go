package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/mascot"
)

func TestRunPlain(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := run([]string{"--plain"}, &out, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit %d, stderr: %s", code, &stderr)
	}
	r := mascot.NewRenderer()
	r.Render(mascot.NeutralPose(), nil)
	if want := r.View() + "\n"; out.String() != want {
		t.Error("--plain must print one shared-renderer frame plus a trailing newline")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestRunPlainWriteFailure(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"--plain"}, failingWriter{}, &stderr); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if got := stderr.String(); !strings.Contains(got, "write preview frame") || !strings.Contains(got, io.ErrClosedPipe.Error()) {
		t.Errorf("stderr = %q, want write context and underlying error", got)
	}
}

func TestRunErrors(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"extra", "--yaw", "10"}, 2, "unexpected arguments"},
		{[]string{"--", "extra"}, 2, "unexpected arguments"},
		{[]string{"--mood="}, 1, "unknown mood"},
		{[]string{"--mood", "angry"}, 1, "unknown mood"},
		{[]string{"--background="}, 1, "background must be #RRGGBB"},
		{[]string{"--background", "#zzzzzz"}, 1, "background must be #RRGGBB"},
		{[]string{"--yaw", "NaN"}, 1, "-yaw must be a finite number"},
		{[]string{"--pitch", "Inf"}, 1, "-pitch must be a finite number"},
		{[]string{"--nope"}, 2, "flag provided but not defined"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var out, stderr bytes.Buffer
			if code := run(append([]string{"--plain"}, tc.args...), &out, &stderr); code != tc.code {
				t.Errorf("exit = %d, want %d", code, tc.code)
			}
			if out.Len() != 0 || !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("stdout=%q stderr=%q, want no frame and %q", &out, &stderr, tc.want)
			}
		})
	}
}

func TestRunFlagOrderIndependent(t *testing.T) {
	r := mascot.NewRenderer()
	pose := mascot.NeutralPose()
	pose.EyeOpen, pose.Lift = 0.5, 0
	r.Render(pose, nil)
	for _, args := range [][]string{
		{"--plain", "--mood", "sleepy", "--eye", "0.5"},
		{"--plain", "--eye", "0.5", "--mood", "sleepy"},
	} {
		var out, stderr bytes.Buffer
		if code := run(args, &out, &stderr); code != 0 || out.String() != r.View()+"\n" {
			t.Errorf("%v: exit %d, stderr %q; explicit eye must override mood", args, code, &stderr)
		}
	}
}

func TestRunInteractive(t *testing.T) {
	savedTTY, savedRun := interactiveTTY, runPreviewProgram
	t.Cleanup(func() { interactiveTTY, runPreviewProgram = savedTTY, savedRun })
	for _, tc := range []struct {
		name         string
		tty, animate bool
		err          error
		code         int
		want         string
	}{
		{"no tty", false, false, nil, 1, "--plain"},
		{"held", true, false, nil, 0, ""},
		{"animated", true, true, nil, 0, ""},
		{"runner failure", true, false, io.ErrClosedPipe, 1, io.ErrClosedPipe.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interactiveTTY = func(io.Writer) bool { return tc.tty }
			called := false
			runPreviewProgram = func(model tea.Model, _ io.Writer) error {
				called = true
				if _, cmd := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24}); (cmd != nil) != tc.animate {
					t.Error("frame clock must match --animate")
				}
				if !strings.Contains(model.View().Content, "yaw +10") {
					t.Error("interactive preview must receive the parsed pose")
				}
				return tc.err
			}
			var out, stderr bytes.Buffer
			args := []string{"--yaw", "10"}
			if tc.animate {
				args = append(args, "--animate")
			}
			if code := run(args, &out, &stderr); code != tc.code {
				t.Errorf("exit = %d, want %d", code, tc.code)
			}
			if called != tc.tty || out.Len() != 0 {
				t.Errorf("runner called=%t, stdout=%q", called, &out)
			}
			if !strings.Contains(stderr.String(), tc.want) || (tc.want == "" && stderr.Len() != 0) {
				t.Errorf("stderr=%q, want %q", &stderr, tc.want)
			}
		})
	}
}
