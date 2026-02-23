#!/usr/bin/env bash
set -euo pipefail

if [[ ! -x ./gh-csd ]]; then
  echo "Build first: go build -o gh-csd ." >&2
  exit 1
fi

tui-qa \
  --cmd "./gh-csd tui" \
  --keys "sleep:1,j,q" \
  --assert "codespace\(s\)"
