# guitar-coach

A local-first CLI and web UI for deliberate guitar practice. It runs timed
practice sessions, tracks BPM progress per exercise (the "six clean reps, then
bump the tempo" rule), keeps per-set notes, and plots your history. No account,
no cloud, no telemetry — everything lives in a local JSON store.

## Features

- Timed sessions: work / rest / break timers with an audible alert (sound,
  volume, beep count and gap are tunable on the command line).
- A warm-up countdown before the first set, skippable with Ctrl-C.
- The rest timer keeps ticking live above the prompts while you record your
  BPM and notes; every prompt is a full line editor with arrow keys.
- BPM prompts pre-fill the suggested tempo — accept with Enter or nudge with
  the up/down arrows.
- Record the BPM you confirmed clean plus notes during the rest window, and
  fix a recording afterwards with `edit`.
- Topics per exercise, with a plan builder that interleaves topics so the same
  one isn't repeated back to back.
- Progress history and a built-in web UI with per-exercise BPM charts.
- Local REST API and shell completions.

## Install

Install the latest released version from source:

```sh
go install github.com/luca-filipponi/guitar-coach@latest
```

Or a specific version:

```sh
go install github.com/luca-filipponi/guitar-coach@v0.1.0
```

To build from a checkout:

```sh
git clone git@github.com:luca-filipponi/guitar-coach.git
cd guitar-coach
make build      # -> bin/guitar-coach
```

There are no prebuilt binary downloads yet — `go install` builds the single
self-contained binary from source, which is enough for a local tool. Ready-made
binaries could be attached to GitHub Releases later via `gh release create`.

## Quick start

```sh
guitar-coach add "alternate picking level 1" --topic "alternate picking"
guitar-coach add "economy picking runs" --topic "economy picking"
guitar-coach list

guitar-coach start        # 5 exercises x 5 min, 1 min rest, 5 min break
guitar-coach serve        # web UI + charts at http://localhost:8080
```

## Commands

| Command | Purpose |
| --- | --- |
| `add <name>... [--topic T]` | add exercises |
| `list` | list exercises |
| `remove <exercise>` | remove an exercise |
| `topic <exercise> [topic]` / `--clear` | show, set or clear a topic |
| `start [--exercises N] [--duration 5m] [--rest 1m] [--break 5m] [--warmup 5m] [--pick a,b] [--select] [--new] [--alarm-sound <file>] [--alarm-volume 1.0-4.0] [--alarm-count 3] [--alarm-gap 220ms]` | run a practice session (asks to reuse the previous rotation unless `--new` is given) |
| `history` / `show <id>` | review past sessions (sessions show their total length) |
| `edit <session> <entry> [--start N] [--end N]` | fix the start/end BPM of a recorded entry (enter numbers match the `show` listing) |
| `seed [--weeks N]` | generate fake history for trying the UI |
| `serve [--port 8080]` | start the local web UI and REST API |
| `version` | print the installed version |
| `completion <shell>` | shell completion script |

## Data

Stored as versioned JSON in `~/.guitar-coach` (override with `--dir <path>` or
the `GUITAR_COACH_DIR` environment variable). Every `--dir` is independent, so
demo data never mixes with your real history.

## Releases & versioning

A release is a Git tag — a permanent name attached to a specific commit. A tag
is not a download: it makes an exact source snapshot reproducible forever, and
it's what `go install ...@latest` and `...@vX.Y.Z` resolve to. (Prebuilt
binaries are optional extras attached to release pages; see Install.)

`make build` embeds the version into the binary through `-ldflags` (see
Makefile). `guitar-coach version` prints it.

To cut a new version (e.g. `v0.1.0`):

```sh
git tag v0.1.0
git push origin master --tags
make build        # the binary now reports v0.1.0
```

Semver: `v0.x.y` — bump `y` for bug fixes, `x` for new features (pre-1.0
convention); `v1.0.0` is the first compatibility promise. Building with
uncommitted changes appends `-dirty` to the version, so every build shows its
provenance.

## Development

```sh
make build   # build to bin/
make test    # go test ./...
make vet     # go vet ./...
make fmt     # gofmt -w .
```

## License

MIT — see [LICENSE](LICENSE).
