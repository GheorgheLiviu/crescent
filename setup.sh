#!/bin/sh
# Crescent on Mac or Linux: open the page at http://localhost:8765 (Ctrl+C stops it).
#   sh setup.sh         open the page
#   sh setup.sh --dev   first install the test dependencies and run the tests (needs Go 1.26+, Node 22 and Python 3)
set -e
cd "$(dirname "$0")"
PORT=8765
if [ "$1" = "--dev" ]; then
  (cd tests && npm install --no-audit --no-fund && npx playwright install chromium)
  sh tests/run.sh
fi
URL="http://localhost:$PORT/"
echo "Crescent page: $URL (Ctrl+C stops it)"
( sleep 1; if command -v open >/dev/null; then open "$URL"; else xdg-open "$URL"; fi ) >/dev/null 2>&1 &
exec python3 -m http.server "$PORT" --bind 127.0.0.1 --directory web
