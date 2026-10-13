# Installs REA (rea.tools), the reverse-engineering tools for AI agents, with npm.
# Needs Node.js 22 or newer: tick Node.js LTS on the page. Safe to run twice (it updates REA).

# Homebrew's node@22 is not linked into bin, so add its folder too.
export PATH="/opt/homebrew/opt/node@22/bin:/usr/local/opt/node@22/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"
if ! command -v npm >/dev/null; then
  echo 'Node.js is not installed, so REA was not. Tick Node.js LTS and run this again.'
  exit 1
fi

# Install into the user's folder, so it needs no administrator password.
npm install --global --prefix "$HOME/.npm-global" rea-agents || exit $?
echo "REA is installed in ~/.npm-global/bin. To connect it to Claude, Cursor or another agent, run: npx rea-agents setup"
