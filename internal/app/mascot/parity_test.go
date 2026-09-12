package mascot

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"strconv"
	"testing"
)

// frozenRigSHA256 is the rig revision ticket 01 froze. Fixtures must record it.
const frozenRigSHA256 = "b8ba26e965ab2aa5bc4f38b6efe7de5b840ee4c5cbca7c1d510f581d7164d24f"

type fixturePose struct {
	Yaw     float64 `json:"yaw"`
	Pitch   float64 `json:"pitch"`
	EyeOpen float64 `json:"eyeOpen"`
	Lift    float64 `json:"lift"`
	Bob     float64 `json:"bob"`
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
	MatchingBackground string        `json:"matchingBackground"`
	Cells              []fixtureCell `json:"cells"`
}

type fixtureFile struct {
	RigSHA256 string        `json:"rigSHA256"`
	Width     int           `json:"width"`
	Height    int           `json:"height"`
	Palette   []*string     `json:"palette"`
	Cases     []fixtureCase `json:"cases"`
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

// TestParityFixtures compares exact rune/FG/BG against the frozen JS rig for
// every generated case. Cells, not ANSI bytes, are the parity unit.
func TestParityFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/rig-v1.json")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var f fixtureFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}
	if f.RigSHA256 != frozenRigSHA256 {
		t.Fatalf("fixture rig hash %s, want frozen %s", f.RigSHA256, frozenRigSHA256)
	}
	if f.Width != Width || f.Height != Height {
		t.Fatalf("fixture dims %dx%d, renderer %dx%d", f.Width, f.Height, Width, Height)
	}
	if len(f.Palette) != 8 || f.Palette[0] != nil {
		t.Fatalf("palette metadata must be 8 entries with index 0 null")
	}

	r := NewRenderer()
	for _, tc := range f.Cases {
		if len(tc.Cells) != Width*Height {
			t.Fatalf("%s: %d cells, want %d", tc.Name, len(tc.Cells), Width*Height)
		}
		bg, err := parseHexRGBA(tc.MatchingBackground)
		if err != nil {
			t.Fatalf("%s: %v", tc.Name, err)
		}
		pose := Pose{tc.Input.Yaw, tc.Input.Pitch, tc.Input.EyeOpen, tc.Input.Lift, tc.Input.Bob}
		r.Render(pose, bg)
		frame := r.Frame()

		diffs := 0
		first := ""
		for i, want := range tc.Cells {
			got := frame[i]
			if got.Glyph == want.Glyph && int(got.FG) == want.FG && int(got.BG) == want.BG {
				continue
			}
			diffs++
			if first == "" {
				first = fmt.Sprintf("cell (%d,%d) [%d]: want %q U+%04X fg=%d bg=%d, got %q U+%04X fg=%d bg=%d",
					i%Width, i/Width, i, want.Glyph, want.Glyph, want.FG, want.BG, got.Glyph, got.Glyph, got.FG, got.BG)
			}
		}
		if diffs > 0 {
			t.Errorf("%s (bg %s): %d/%d cells differ; first: %s", tc.Name, tc.MatchingBackground, diffs, len(tc.Cells), first)
		}
	}
}
