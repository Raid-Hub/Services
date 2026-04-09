#!/usr/bin/env bash
# Run `go` inside golang:1.25-alpine with the repo at /src.
# Use when the host cannot install Go 1.25 (see docs/LOCAL_DEVELOPMENT.md).
# Example: ./scripts/go-docker.sh build -o /dev/null ./apps/hermes/

set -euo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
exec docker run --rm \
  -v "${repo_root}:/src" \
  -w /src \
  golang:1.25-alpine \
  go "$@"
