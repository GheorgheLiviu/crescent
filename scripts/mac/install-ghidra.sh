# Installs Ghidra (ghidra-sre.org), the NSA's reverse engineering tool, with Homebrew.
# Homebrew also installs the Java it needs. Safe to run twice.

export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"
if ! command -v brew >/dev/null; then
  echo 'Homebrew is not installed, so Ghidra was not.'
  exit 1
fi

brew install --cask ghidra || exit $?
echo 'Ghidra is installed. Run it with: ghidraRun'
