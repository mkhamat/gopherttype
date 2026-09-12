package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/mascot"
)

func TestRunOneShot(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"--plain"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errBuf.String())
	}
	if n := strings.Count(out.String(), "\n"); n != 12 {
		t.Errorf("want 12 newlines (11 internal + trailing), got %d", n)
	}
	if errBuf.Len() != 0 {
		t.Errorf("unexpected stderr: %s", errBuf.String())
	}
}

// TestRunPlainMatchesRenderer pins the one-shot path to the shared renderer:
// one frame plus a trailing newline, byte-for-byte.
func TestRunPlainMatchesRenderer(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"--plain", "--mood", "calm"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errBuf.String())
	}
	r := mascot.NewRenderer()
	r.Render(mascot.NeutralPose(), nil)
	if want := r.View() + "\n"; out.String() != want {
		t.Error("--plain output must equal one rendered frame plus a trailing newline")
	}
}

func TestRunFlagOrderIndependent(t *testing.T) {
	var a, b, ea, eb bytes.Buffer
	if code := run([]string{"--plain", "--mood", "sleepy", "--eye", "0.5"}, &a, &ea); code != 0 {
		t.Fatalf("first exit %d: %s", code, ea.String())
	}
	if code := run([]string{"--plain", "--eye", "0.5", "--mood", "sleepy"}, &b, &eb); code != 0 {
		t.Fatalf("second exit %d: %s", code, eb.String())
	}
	if a.String() != b.String() {
		t.Error("explicit --eye must override mood regardless of flag order")
	}
}

func TestRunErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--mood", "angry"},
		{"--background", "#12345"},
		{"--background", "red"},
		{"--yaw", "NaN"},
		{"--pitch", "Inf"},
		{"--nope"},
	} {
		var out, errBuf bytes.Buffer
		if code := run(append([]string{"--plain"}, args...), &out, &errBuf); code == 0 {
			t.Errorf("%v: expected nonzero exit", args)
		}
		if out.Len() != 0 {
			t.Errorf("%v: stdout should stay empty, got %q", args, out.String())
		}
	}
}

// TestRunDefaultRequiresTTY checks the default (interactive) invocation fails
// clearly without a terminal instead of writing an alternate screen into a pipe.
func TestRunDefaultRequiresTTY(t *testing.T) {
	saved := interactiveTTY
	interactiveTTY = func(io.Writer) bool { return false }
	defer func() { interactiveTTY = saved }()

	var out, errBuf bytes.Buffer
	if code := run(nil, &out, &errBuf); code == 0 {
		t.Fatal("interactive default without a TTY must fail")
	}
	if out.Len() != 0 {
		t.Errorf("stdout should stay empty, got %q", out.String())
	}
	if !strings.Contains(errBuf.String(), "--plain") {
		t.Errorf("error should mention --plain, got %q", errBuf.String())
	}
}

// TestRunInteractiveSelectsPreview checks a terminal builds the preview model
// (with the parsed pose) and hands it to Bubble Tea.
func TestRunInteractiveSelectsPreview(t *testing.T) {
	savedTTY, savedRun := interactiveTTY, runPreviewProgram
	defer func() { interactiveTTY, runPreviewProgram = savedTTY, savedRun }()

	interactiveTTY = func(io.Writer) bool { return true }
	var got tea.Model
	runPreviewProgram = func(model tea.Model, _ io.Writer) error {
		got = model
		return nil
	}

	var out, errBuf bytes.Buffer
	if code := run([]string{"--yaw", "10"}, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errBuf.String())
	}
	if got == nil {
		t.Fatal("interactive run should build a preview model")
	}
	if out.Len() != 0 {
		t.Errorf("interactive mode should not print a one-shot frame, got %q", out.String())
	}
}

// TestRunInteractiveErrorPropagates reports a runner failure at exit.
func TestRunInteractiveErrorPropagates(t *testing.T) {
	savedTTY, savedRun := interactiveTTY, runPreviewProgram
	defer func() { interactiveTTY, runPreviewProgram = savedTTY, savedRun }()

	interactiveTTY = func(io.Writer) bool { return true }
	runPreviewProgram = func(tea.Model, io.Writer) error { return errors.New("boom") }

	var out, errBuf bytes.Buffer
	if code := run(nil, &out, &errBuf); code == 0 {
		t.Fatal("runner error must be nonzero")
	}
	if !strings.Contains(errBuf.String(), "boom") {
		t.Errorf("stderr should contain the runner error, got %q", errBuf.String())
	}
}
