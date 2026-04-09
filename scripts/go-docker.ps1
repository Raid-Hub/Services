# Run `go` inside golang:1.25-alpine with the repo at /src.
# Use when the host cannot install Go 1.25 (see docs/LOCAL_DEVELOPMENT.md).
# Example: .\scripts\go-docker.ps1 build -o $null .\apps\hermes\

param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$GoArgs
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot

docker run --rm `
    -v "${repoRoot}:/src" `
    -w /src `
    golang:1.25-alpine `
    go @GoArgs
