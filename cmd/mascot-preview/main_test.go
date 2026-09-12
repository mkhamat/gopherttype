package main

import (
	"bytes"
	"strings"
	"testing"
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

func TestRunFlagOrderIndependent(t *testing.T) {
	var a, b, ea, eb bytes.Buffer
	if code := run([]string{"--mood", "sleepy", "--eye", "0.5"}, &a, &ea); code != 0 {
		t.Fatalf("first exit %d: %s", code, ea.String())
	}
	if code := run([]string{"--eye", "0.5", "--mood", "sleepy"}, &b, &eb); code != 0 {
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
		if code := run(args, &out, &errBuf); code == 0 {
			t.Errorf("%v: expected nonzero exit", args)
		}
		if out.Len() != 0 {
			t.Errorf("%v: stdout should stay empty, got %q", args, out.String())
		}
	}
}
