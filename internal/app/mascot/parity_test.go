package mascot

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"os"
	"strconv"
	"testing"
)

// frozenRigSHA256 is the rig revision tickets 01/02 froze. Fixtures must record it.
const frozenRigSHA256 = "b8ba26e965ab2aa5bc4f38b6efe7de5b840ee4c5cbca7c1d510f581d7164d24f"

type fixturePose struct {
	Yaw     float64 `json:"yaw"`
	Pitch   float64 `json:"pitch"`
	EyeOpen float64 `json:"eyeOpen"`
	Lift    float64 `json:"lift"`
	Bob     float64 `json:"bob"`
}

func (p fixturePose) pose() Pose {
	return Pose{Yaw: p.Yaw, Pitch: p.Pitch, EyeOpen: p.EyeOpen, Lift: p.Lift, Bob: p.Bob}
}

type fixtureCell struct {
	Glyph rune
	FG    int
	BG    int
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
	Name               string        `json:"name"`
	Input              fixturePose   `json:"input"`
	Pose               fixturePose   `json:"pose"`
	MatchingBackground string        `json:"matchingBackground"`
	Omitted            []string      `json:"omitted"`
	Cells              []fixtureCell `json:"cells"`
}

type fixtureFrame struct {
	I     int           `json:"i"`
	T     int           `json:"t"`
	Input fixturePose   `json:"input"`
	Pose  fixturePose   `json:"pose"`
	Cells []fixtureCell `json:"cells"`
}

type fixtureFile struct {
	Schema             int            `json:"schema"`
	Set                string         `json:"set"`
	RigSHA256          string         `json:"rigSHA256"`
	Width              int            `json:"width"`
	Height             int            `json:"height"`
	Palette            []*string      `json:"palette"`
	MatchingBackground string         `json:"matchingBackground"`
	Cases              []fixtureCase  `json:"cases"`
	Frames             []fixtureFrame `json:"frames"`
	Seam               []fixtureFrame `json:"seam"`
}

func parseHexRGBA(s string) (color.RGBA, error) {
	if len(s) != 7 || s[0] != '#' {
		return color.RGBA{}, fmt.Errorf("want #RRGGBB, got %q", s)
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("want #RRGGBB, got %q", s)
	}
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}, nil
}

// loadFixture reads and validates a generated fixture manifest.
func loadFixture(t *testing.T, path string) fixtureFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var f fixtureFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}
	if f.RigSHA256 != frozenRigSHA256 {
		t.Fatalf("%s: fixture rig hash %s, want frozen %s", path, f.RigSHA256, frozenRigSHA256)
	}
	if f.Width != Width || f.Height != Height {
		t.Fatalf("%s: fixture dims %dx%d, renderer %dx%d", path, f.Width, f.Height, Width, Height)
	}
	if len(f.Palette) != 8 || f.Palette[0] != nil {
		t.Fatalf("%s: palette metadata must be 8 entries with index 0 null", path)
	}
	return f
}

// roleName and partName make parity diagnostics readable.
func roleName(r role) string {
	switch r {
	case roleBackground:
		return "bg"
	case roleFur:
		return "fur"
	case roleEye:
		return "eye"
	case rolePupil:
		return "pupil"
	case roleNose:
		return "nose"
	case roleMouth:
		return "mouth"
	case roleTooth:
		return "tooth"
	default:
		return "?"
	}
}

func partName(p partID) string {
	switch p {
	case partBody:
		return "body"
	case partHead:
		return "head"
	case partEarLeft:
		return "ear-left"
	case partEarRight:
		return "ear-right"
	case partEyeLeft:
		return "eye-left"
	case partEyeRight:
		return "eye-right"
	case partMuzzle:
		return "muzzle"
	case partNose:
		return "nose"
	case partToothLeft:
		return "tooth-left"
	case partToothRight:
		return "tooth-right"
	default:
		return "?"
	}
}

// diagnoseCell names the eight samples that make up a differing cell so a
// mismatch can be traced to a hit/material rather than just a coordinate.
func diagnoseCell(r *Renderer, i int, want fixtureCell, got Cell) string {
	y, x := i/Width, i%Width
	var s [8]sample
	n := 0
	for dy := 0; dy < 4; dy++ {
		for dx := 0; dx < 2; dx++ {
			s[n] = r.samples[(y*4+dy)*SampleWidth+x*2+dx]
			n++
		}
	}
	desc := fmt.Sprintf("cell (%d,%d) [%d]: want %q U+%04X fg=%d bg=%d, got %q U+%04X fg=%d bg=%d\n    samples:",
		x, y, i, want.Glyph, want.Glyph, want.FG, want.BG, got.Glyph, got.Glyph, got.FG, got.BG)
	for k, smp := range s {
		desc += fmt.Sprintf(" [%d c=%d %s/%s under=%d x=%.4f]", k, smp.color, roleName(smp.role), partName(smp.part), smp.under, smp.x)
	}
	return desc
}

// checkCells renders one pose at a background and reports exact cell diffs.
func checkCells(t *testing.T, r *Renderer, label, bgHex string, pose Pose, want []fixtureCell) int {
	t.Helper()
	if len(want) != Width*Height {
		t.Fatalf("%s: %d cells, want %d", label, len(want), Width*Height)
	}
	bg, err := parseHexRGBA(bgHex)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	r.Render(pose, bg)
	frame := r.Frame()

	diffs := 0
	first := ""
	for i, w := range want {
		got := frame[i]
		if got.Glyph == w.Glyph && int(got.FG) == w.FG && int(got.BG) == w.BG {
			continue
		}
		diffs++
		if first == "" {
			first = diagnoseCell(r, i, w, got)
		}
	}
	if diffs > 0 {
		t.Errorf("%s (bg %s): %d/%d cells differ; first:\n    %s", label, bgHex, diffs, len(want), first)
	}
	return diffs
}

// checkSanitizedPose confirms Go's sanitizer agrees with the recorded JS output
// pose for a case (clamping, neutral fallback, omitted-field mapping).
func checkSanitizedPose(t *testing.T, label string, input, want fixturePose) {
	t.Helper()
	got := sanitizePose(input.pose())
	w := want.pose()
	const eps = 1e-9
	if math.Abs(got.Yaw-w.Yaw) > eps || math.Abs(got.Pitch-w.Pitch) > eps ||
		math.Abs(got.EyeOpen-w.EyeOpen) > eps || math.Abs(got.Lift-w.Lift) > eps || math.Abs(got.Bob-w.Bob) > eps {
		t.Errorf("%s: sanitized pose %+v, want %+v", label, got, w)
	}
}

// TestParityFixtures compares exact rune/FG/BG against the frozen JS rig for
// the full static grid and every named boundary case. Cells, not ANSI bytes,
// are the parity unit.
func TestParityFixtures(t *testing.T) {
	f := loadFixture(t, "testdata/rig-v1.json")
	if len(f.Cases) != 375+36 {
		t.Fatalf("rig-v1: %d cases, want 375 grid + 36 boundary", len(f.Cases))
	}

	r := NewRenderer()
	for _, tc := range f.Cases {
		checkSanitizedPose(t, tc.Name, tc.Input, tc.Pose)
		checkCells(t, r, tc.Name, tc.MatchingBackground, tc.Input.pose(), tc.Cells)
	}
}

// TestParityLoop compares the 288 stored demonstration poses exactly and checks
// the t=0/t=12 seam without appending a duplicate final frame.
func TestParityLoop(t *testing.T) {
	f := loadFixture(t, "testdata/loop-v1.json")
	if f.Set != "loop-v1" {
		t.Fatalf("loop fixture set = %q, want loop-v1", f.Set)
	}
	if len(f.Frames) != 288 {
		t.Fatalf("loop fixture has %d frames, want 288", len(f.Frames))
	}
	if f.MatchingBackground == "" {
		t.Fatal("loop fixture missing matchingBackground")
	}

	r := NewRenderer()
	for i, fr := range f.Frames {
		if fr.I != i {
			t.Fatalf("frame %d has index %d", i, fr.I)
		}
		checkSanitizedPose(t, fmt.Sprintf("loop-%d", i), fr.Input, fr.Pose)
		checkCells(t, r, fmt.Sprintf("loop-%d", i), f.MatchingBackground, fr.Input.pose(), fr.Cells)
	}

	// Seam: t=0 and t=12 are the same pose modulo float noise; their cells must
	// match exactly. The clip stores 0..287, so t=12 is where a 289th frame
	// would land -- it must equal the first frame, not be appended.
	if len(f.Seam) != 2 {
		t.Fatalf("loop fixture seam has %d entries, want 2", len(f.Seam))
	}
	for _, fr := range f.Seam {
		checkSanitizedPose(t, fmt.Sprintf("seam-t%d", fr.T), fr.Input, fr.Pose)
		checkCells(t, r, fmt.Sprintf("seam-t%d", fr.T), f.MatchingBackground, fr.Input.pose(), fr.Cells)
	}
	if !cellsEqual(f.Frames[0].Cells, f.Seam[0].Cells) {
		t.Error("seam t=0 cells differ from stored frame 0")
	}
	if !cellsEqual(f.Seam[0].Cells, f.Seam[1].Cells) {
		t.Error("seam t=0 and t=12 cells differ; loop is not periodic")
	}
}

func cellsEqual(a, b []fixtureCell) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
