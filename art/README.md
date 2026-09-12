# Poseable gopher art

A 32×12 colored-character study of the classic Go gopher, using the supplied `gopher-reference.png`: shaded cyan fur, big white eyes with solid dark pupils, dark ear centers, one oval black nose, a tan two-lobed muzzle and two rounded buck teeth. Contours and shading use solid quadrant blocks only—no Braille dots, stippling or texture dithering.

**This is a new art revision, not native Go integration or terminal approval.** The Go renderer, its existing fixtures, and the older frozen-art descriptions under `plans/` have not been updated to this look. Review the art before porting or rebaselining those fixtures.

## Watch it in your terminal

From the repository root, with Node.js available:

```sh
node art/mascot-export.mjs --play
```

Ctrl-C exits and restores the terminal. Playback needs at least 32×14 cells: 12 art rows plus status space. The demo targets 24 FPS; it makes no claim about application performance.

For a light terminal, use its approximate background color for palette matching:

```sh
node art/mascot-export.mjs --play --background '#faf8f0'
```

Index 0 remains the terminal default. Facial colors do not change with the theme.

## Interactive art preview

```sh
python3 -m http.server 8765 --bind 127.0.0.1 --directory art
```

Open <http://127.0.0.1:8765/mascot-rig.html>. Stop the server with Ctrl-C. ES modules require serving the files rather than double-clicking the HTML.

The live character rig sits beside the supplied reference. Controls cover whole-head yaw ±35°, downward pitch 0–20°, eye aperture, five moods, a single blink and a turn/blink loop. Neutral faces straight ahead at pitch 0°. The body stays fixed; ears and facial features rotate with the head, while the light stays in world space.

The default browser view draws the encoded quarter-cell blocks directly with CSS, avoiding font gaps that can split pupils and teeth into stripes. **Use font glyphs** switches back to actual characters for comparison. Both modes show exactly the same cell data; neither substitutes a smoother illustration. Check the ANSI player for your actual terminal's appearance.

## Files

| File | Purpose |
| --- | --- |
| `gopher-reference.png` | User-supplied visual target; its checkerboard is part of the image |
| `mascot-rig.mjs` | Ellipsoid and rounded-plate geometry, five-tone lighting, expressions, solid-cell encoding, ANSI and SVG serialization |
| `mascot-rig.html` | Interactive preview and downloads |
| `mascot-export.mjs` | Terminal player and command-line exporter |
| `mascot-loop.json` | Generated 12-second calm loop: 288 frames at 24 FPS |
| `mascot-preview.svg` / `.png` | Generated neutral cell-geometry previews of the current rig, not the old accepted study |
| `mascot-fixtures.mjs` | Development-only fixture generator; do not overwrite native fixtures without reviewing and porting the new rig |

## Export

```sh
node art/mascot-export.mjs --yaw -25 --pitch 18 --mood proud
node art/mascot-export.mjs --json --yaw 25 > /tmp/gopher-frame.json
node art/mascot-export.mjs --svg > /tmp/gopher-frame.svg
node art/mascot-export.mjs --rig > /tmp/gopher-geometry.json
node art/mascot-export.mjs --clip > /tmp/gopher-loop.json
```

Default output is an ANSI frame. `--clip` drives yaw/pitch itself and accepts `--mood` and `--background`. Geometry JSON is only geometry/pose data; material and encoder rules remain in `mascot-rig.mjs`.

Regenerate the included assets after editing the rig (PNG conversion requires ImageMagick):

```sh
node art/mascot-export.mjs --clip > art/mascot-loop.json
node art/mascot-export.mjs --svg > art/mascot-preview.svg
magick art/mascot-preview.svg art/mascot-preview.png
```

SVG previews draw the actual encoded quadrants, without smoother facial overlays that the terminal cannot reproduce. They are geometry studies, not pixel-exact terminal-font screenshots.

### Frame format

- `version`: 1; `width`, `height`: 32, 12.
- `palette`: **nine entries**. Index 0 is `null` (terminal default); 1–5 are cyan shadows through highlights, 6 is white, 7 dark ink, and 8 the tan muzzle. Broad lighting bands replace the previous flat fill; eyes and teeth stay white.
- `matchingBackground`: RGB approximation for quantization, never a request to paint the surrounding terminal rectangle.
- `pose`: yaw/pitch in degrees, eye openness 0–1, muzzle lift and head bob in world units.
- `cells`: exactly 384 `[glyph, foregroundIndex, backgroundIndex]` triples, row-major. Cell `(x,y)` is at `y*32+x`.
- Loop exports add `fps`, `loop`, `mood` and `frames`; every frame has its own `pose` and `cells`.

**Keep colored spaces.** A space with nonzero background is filled artwork, not padding. Apply both color channels.

Frame time is `frameIndex/fps`. There is no duplicate final frame; playback uses elapsed time rather than advancing geometry by frame number.

## Tuning and limits

Edit `rig` for proportions, `material` for lighting/lids and muzzle colors, `moods` for expression targets, and `loopPose` for choreography.

- The unchanged 32×12 output uses 128×96 ray samples. Each solid quarter-cell integrates eight samples; the encoder chooses its two colors and quadrant mask together. This reduces tiny coverage changes without adding stippling.
- Pupils are solid head-local ellipsoids, not speckled surface outlines. The lids cover their parent eyes and pupils together.
- Teeth are two filled, rounded rectangular front plates with a narrow dark separator. All three share the head transform. Lift moves the muzzle and mouth, not the tooth divider.
- Upper-left lighting uses five cyan tones. Head, cheeks, body and lids use a common smooth shading envelope to avoid harsh joins between overlapping parts.

At 32×12, curved geometry still becomes stepped character edges. Thin black outlines and tiny pupil catchlights are intentionally omitted instead of becoming isolated pixels. Inspect turns, blinks and both terminal backgrounds at normal font size before accepting the translation.

This allocating JavaScript prototype is an editable art tool, not a production performance model. A future native Go port must evaluate arbitrary poses procedurally; Node, browser rendering and prerecorded JSON clips do not belong in the application runtime. The integration sequence in [`../PLAN.md`](../PLAN.md) still requires explicit native-terminal visual approval, but its old geometry/palette baseline must be revised deliberately after art review.

Original Go gopher design by [Renee French](https://go.dev/blog/gopher), [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), adapted into this terminal art study.
