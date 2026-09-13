package mascot

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"slices"
	"strconv"
	"testing"
)

const frozenRigSHA256 = "c4d542faa6f01f4e0f2665a0414db1539f7b11eb23b1e7baa6196d58955bcabf"

type fixtureCell struct {
	Glyph  rune
	FG, BG int
}

func (c *fixtureCell) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if len(raw) != 3 {
		return fmt.Errorf("want 3 cell fields, got %d", len(raw))
	}
	var glyph string
	if err := json.Unmarshal(raw[0], &glyph); err != nil {
		return err
	}
	runes := []rune(glyph)
	if len(runes) != 1 {
		return fmt.Errorf("want single-rune glyph, got %q", glyph)
	}
	c.Glyph = runes[0]
	if err := json.Unmarshal(raw[1], &c.FG); err != nil {
		return err
	}
	return json.Unmarshal(raw[2], &c.BG)
}

type fixtureCase struct {
	Name               string
	Input, Pose        Pose
	MatchingBackground string
	Cells              []fixtureCell
}

type fixtureFrame struct {
	I, T        int
	Input, Pose Pose
	Cells       []fixtureCell
}

type fixtureFile struct {
	Schema             int
	Set, RigSHA256     string
	Width, Height      int
	Palette            []*string
	MatchingBackground string
	Cases              []fixtureCase
	Frames, Seam       []fixtureFrame
}

func loadFixture(t *testing.T, set string) fixtureFile {
	t.Helper()
	data, err := os.ReadFile("testdata/" + set + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixtureFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if f.Schema != 1 || f.Set != set || f.RigSHA256 != frozenRigSHA256 {
		t.Fatalf("%s: unexpected fixture schema, set or frozen rig hash", set)
	}
	if f.Width != Width || f.Height != Height || len(f.Palette) != 9 || f.Palette[0] != nil {
		t.Fatalf("%s: invalid dimensions or palette metadata", set)
	}
	return f
}

func checkFixture(t *testing.T, r *Renderer, label, bgHex string, input, pose Pose, cells []fixtureCell) {
	t.Helper()
	if got := sanitizePose(input); got != pose {
		t.Errorf("%s: sanitized pose %+v, want %+v", label, got, pose)
	}
	if len(cells) != Width*Height {
		t.Fatalf("%s: %d cells, want %d", label, len(cells), Width*Height)
	}
	if len(bgHex) != 7 || bgHex[0] != '#' {
		t.Fatalf("%s: invalid background %q", label, bgHex)
	}
	v, err := strconv.ParseUint(bgHex[1:], 16, 32)
	if err != nil {
		t.Fatal(err)
	}
	r.Render(input, color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff})
	frame := r.Frame()
	diffs, first := 0, ""
	for i, want := range cells {
		got := frame[i]
		if got.Glyph == want.Glyph && int(got.FG) == want.FG && int(got.BG) == want.BG {
			continue
		}
		diffs++
		if first == "" {
			first = fmt.Sprintf("(%d,%d): want %q U+%04X fg=%d bg=%d, got %q U+%04X fg=%d bg=%d",
				i%Width, i/Width, want.Glyph, want.Glyph, want.FG, want.BG, got.Glyph, got.Glyph, got.FG, got.BG)
		}
	}
	if diffs != 0 {
		t.Errorf("%s (bg %s): %d/%d cells differ; first %s", label, bgHex, diffs, len(cells), first)
	}
}

func TestParityFixtures(t *testing.T) {
	f := loadFixture(t, "rig-v1")
	if len(f.Cases) != 411 {
		t.Fatalf("%d cases, want 375 grid + 36 boundary", len(f.Cases))
	}
	r := NewRenderer()
	for _, tc := range f.Cases {
		checkFixture(t, r, tc.Name, tc.MatchingBackground, tc.Input, tc.Pose, tc.Cells)
	}
}

func TestParityLoop(t *testing.T) {
	f := loadFixture(t, "loop-v1")
	if len(f.Frames) != 288 || len(f.Seam) != 2 {
		t.Fatal("want 288 frames and a separate two-frame seam")
	}
	r := NewRenderer()
	for i, fr := range f.Frames {
		if fr.I != i {
			t.Fatalf("frame %d has index %d", i, fr.I)
		}
		checkFixture(t, r, fmt.Sprintf("loop-%d", i), f.MatchingBackground, fr.Input, fr.Pose, fr.Cells)
	}
	for i, fr := range f.Seam {
		if fr.T != i*12 {
			t.Fatalf("seam entry %d has time %d, want %d", i, fr.T, i*12)
		}
		checkFixture(t, r, fmt.Sprintf("seam-t%d", fr.T), f.MatchingBackground, fr.Input, fr.Pose, fr.Cells)
		if !slices.Equal(f.Frames[0].Cells, fr.Cells) {
			t.Errorf("seam t=%d differs from frame 0", fr.T)
		}
	}
}
