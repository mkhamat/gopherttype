# gopherttype

A terminal typing game with word-count and timed rounds, watched by a shaded Go
gopher mascot that follows your input and reacts to how you type.

## Run the game

```sh
go run ./cmd/gopherttype
```

The game runs in the alternate screen and needs a real terminal.

- **Home:** `↑`/`↓` (or `k`/`j`) switch mode (time / words), `←`/`→` (or `h`/`l`)
  pick a length, `Enter` begins, `q` quits.
- **Play:** type the highlighted word, `Space` advances, `Backspace` edits,
  `Ctrl+Backspace` or `Alt+Backspace` deletes a word, `Esc` returns home.
- **Results:** `Enter` retry, `Tab` home, `q` quits.
- `Ctrl+C` quits from any screen.

The mascot is keyboard-only (no mouse): the whole head turns toward the selected
home option and the play caret, flinches on a typo, worries after repeated
mistakes, brightens on fast accurate typing, gets sleepy during a pause, and
shows a single expression for the finished round. Its reactions are cosmetic and
never change scoring, the deadline, or navigation.

## Mascot visibility

The mascot is one fixed **32×12** block centred above the controls. It is shown
whole or hidden — never cropped, scaled, wrapped or compacted — and when hidden
its rows are reclaimed (no blank gap). It needs width ≥ 32 and enough height
(about 24 rows for home, 19 for play). Below the minimum window (28×12) the game
shows a resize hint. Colored spaces are filled artwork, not padding.

## Terminal, font and color

The mascot needs a terminal with **truecolor** (or a 256-color downgrade) and a
font that renders the Unicode quadrant/block glyphs (`▀ ▄ ▌ ▐ █` and friends).
It leaves the terminal background alone by default; a matching background color
is used only to quantize cell edges, never to paint a rectangle. There is no
automatic font detection and no hide setting.

**What was actually tested:** only this developer machine (macOS, Apple M4 Pro,
`go1.26.2 darwin/arm64`) using the preview below. No specific terminal, font,
color profile, 256-color, tmux/SSH or Linux setup has been recorded or visually
approved here, so no blanket compatibility claim is made — check it on yours.

## Native mascot preview (developer tool)

One frame, no terminal required:

```sh
go run ./cmd/mascot-preview --plain --yaw 0 --pitch 14 --mood calm
```

Interactive (needs a terminal at least 32×14), held pose or animated demo:

```sh
go run ./cmd/mascot-preview
go run ./cmd/mascot-preview --animate
```

Preview keys: `←`/`→` yaw (±35°), `↑`/`↓` pitch (0–20°), `m` mood cycle
(calm → proud → worried → sleepy → flinch), `a` toggle motion, `b` blink once,
`r` reset to neutral, `q`/`Esc`/`Ctrl+C` quit. Flags `--yaw`, `--pitch`,
`--mood`, `--eye`, `--lift`, `--bob` and `--background #RRGGBB` set a held pose
and are clamped to the rig's limits.

## Runtime

The gopher is rendered procedurally with analytic ray intersections on a single
120 Hz target clock; the same renderer drives the preview and every screen. No
SVG, PNG, JSON clip, image protocol, Node or browser is loaded at runtime, and
none of `art/` is required to build or run either binary (`--plain` works with
no TTY). The `art/` tools remain a development reference, not the shipped
renderer.

## Attribution

Go gopher original design by Renee French; adapted here into a shaded terminal
mascot. [Original](https://go.dev/blog/gopher) · [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).
