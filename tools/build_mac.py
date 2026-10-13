"""Builds the Mac installer program for the page: python tools/build_mac.py

Compiles installer/ for Apple Silicon and Intel Macs, joins both into one universal program
(a "fat" Mach-O file, what Apple's lipo makes) and writes it to web/installer-mac.js, which the
page loads when Mac is picked. Go signs each half ad hoc, which Apple Silicon Macs require.
"""
import base64
import os
import pathlib
import struct
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
# (GOARCH, Mach-O CPU type, CPU subtype, alignment as a power of 2)
ARCHES = [("arm64", 0x0100000C, 0, 14), ("amd64", 0x01000007, 3, 12)]


def build(arch, out):
    env = dict(os.environ, GOOS="darwin", GOARCH=arch, CGO_ENABLED="0")
    subprocess.run(["go", "build", "-trimpath", "-ldflags", "-s -w", "-o", str(out), "."],
                   cwd=ROOT / "installer", env=env, check=True)
    return out.read_bytes()


def fat(slices):
    """slices: [(cputype, subtype, align, bytes)] -> one universal Mach-O file."""
    head = struct.pack(">II", 0xCAFEBABE, len(slices))
    offset = 8 + 20 * len(slices)
    entries, body = b"", b""
    for cpu, sub, align, data in slices:
        start = -(-offset // (1 << align)) * (1 << align)
        body += b"\0" * (start - offset) + data
        entries += struct.pack(">IIIII", cpu, sub, start, len(data), align)
        offset = start + len(data)
    return head + entries + body


def main():
    with tempfile.TemporaryDirectory() as tmp:
        slices = [(cpu, sub, align, build(arch, pathlib.Path(tmp) / arch)) for arch, cpu, sub, align in ARCHES]
    exe = fat(slices)
    (ROOT / "web" / "installer-mac.js").write_bytes(
        b'window.CRESCENT_MAC_B64="' + base64.b64encode(exe) + b'";\n')
    print(f"web/installer-mac.js: {len(exe) / 1048576:.1f} MB program")


if __name__ == "__main__":
    main()
