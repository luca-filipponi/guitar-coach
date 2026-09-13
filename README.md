# guitar-coach

A local-first CLI and web UI for deliberate guitar practice. It runs timed
practice sessions, tracks BPM progress per exercise (the "six clean reps, then
bump the tempo" rule), keeps per-set notes, and plots your history. No account,
no cloud, no telemetry — everything lives in a local JSON store.

## Features

- Timed sessions: work / rest / break timers with an audible alert.
- Record the BPM you confirmed clean plus notes during the rest window.
- Topics per exercise, with a plan builder that interleaves topics so the same
  one isn't repeated back to back.
- Progress history and a built-in web UI with per-exercise BPM charts.
- Local REST API and shell completions.

## Install

```sh
go install github.com/luca-filipponi/guitar-coach@latest
```

Or build from source:

```sh
git clone git@github.com:luca-filipponi/guitar-coach.git
cd guitar-coach
make build      # -> bin/guitar-coach
```

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
| `start [--exercises N] [--duration 5m] [--rest 1m] [--break 5m] [--pick a,b] [--select] [--new]` | run a practice session (asks to reuse the previous rotation unless `--new` is given) |
| `history` / `show <id>` | review past sessions |
| `seed [--weeks N]` | generate fake history for trying the UI |
| `serve [--port 8080]` | start the local web UI and REST API |
| `version` | print the installed version |
| `completion <shell>` | shell completion script |

## Data

Stored as versioned JSON in `~/.guitar-coach` (override with `--dir <path>` or
the `GUITAR_COACH_DIR` environment variable). Every `--dir` is independent, so
demo data never mixes with your real history.

## Development

```sh
make build   # build to bin/
make test    # go test ./...
make vet     # go vet ./...
make fmt     # gofmt -w .
```

## License

MIT — see [LICENSE](LICENSE).
