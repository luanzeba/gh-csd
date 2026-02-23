# AGENTS

## Build & Test
- gofmt: `gofmt -w .`
- tests: `go test ./...`
- build: `go build -o gh-csd .`

## TUI smoke test
- Build the binary first, then run: `scripts/tui-qa.sh`
  - Uses `tui-qa` (from dotfiles) to drive the TUI in a PTY.

## Notes
- Requires `gh` auth and access to Codespaces for TUI data.
