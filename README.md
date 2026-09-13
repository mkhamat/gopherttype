# gopherttype

A terminal typing game with word-count and timed rounds, watched by a Go
gopher mascot that follows your input and reacts to how you type.

## Run the game

Requires Go 1.26.2

```sh
go run ./cmd/gopherttype
```

or build

```sh
go build ./cmd/gopherttype
./gopherttype
```

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

## Runtime

The gopher is rendered procedurally with analytic ray intersections on a single
120 Hz target clock; the same renderer drives the preview and every screen. No
SVG, PNG, JSON clip, image protocol, Node or browser is loaded at runtime.

## Development

```sh
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build ./...
go mod tidy -diff
```

## Attribution

Go gopher original design by Renee French; adapted here into a shaded terminal
mascot. [Original](https://go.dev/blog/gopher) · [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).
