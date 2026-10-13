#!/bin/sh
# Runs every Crescent test with one command: sh tests/run.sh (or npm test in this folder).
# Needs Go and Node 22 with Playwright (npm install here, then npx playwright install chromium).
set -e
cd "$(dirname "$0")/.."

echo "== Installer program (Go)"
(cd installer && go test ./... && GOOS=windows go vet . && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /dev/null . && GOOS=darwin GOARCH=arm64 go vet .)

echo "== Page, installer window and end to end (browser)"
node --test tests/*.test.mjs
