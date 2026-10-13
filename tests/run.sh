#!/bin/sh
# Runs every Crescent test with one command: sh tests/run.sh (or npm test in this folder).
# Needs Go and Node 22 with Playwright (npm install here, then npx playwright install chromium).
set -e
cd "$(dirname "$0")/.."

echo "== Installer program (Go)"
# On Windows the tests run as 32-bit: the 64-bit build carries the admin manifest, which go test can't start.
(cd installer && GOARCH=$([ "$(go env GOOS)" = windows ] && echo 386 || go env GOARCH) go test ./... && GOOS=windows go vet . && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /dev/null . && GOOS=darwin GOARCH=arm64 go vet .)

echo "== Page, installer window and end to end (browser)"
node --test tests/*.test.mjs
