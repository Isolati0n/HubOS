#!/bin/bash
# check-libs.sh ROOT: run ldd (inside ROOT, with chroot) on every ELF file under ROOT/usr, ROOT/bin, ROOT/sbin and
# ROOT/lib and fail (exit 1) if any shared library is "not found". Needs root. Statically linked files are fine.
# Prints the number of files checked.
set -e
ROOT=$(cd "${1:?usage: check-libs.sh ROOT}" && pwd)
LIST=$(mktemp); OUT=$(mktemp)
trap 'rm -f "$LIST" "$OUT"' EXIT
python3 - "$ROOT" > "$LIST" <<'PY'
import os, sys
root = sys.argv[1]
for top in ("usr", "bin", "sbin", "lib"):
    d = os.path.join(root, top)
    if os.path.islink(d) or not os.path.isdir(d):
        continue            # /bin, /sbin, /lib are links into /usr in this root; /usr covers them
    for dp, dn, fn in os.walk(d):
        for n in fn:
            p = os.path.join(dp, n)
            if os.path.islink(p) or not os.path.isfile(p):
                continue
            try:
                with open(p, "rb") as f:
                    if f.read(4) != b"\x7fELF":
                        continue
            except OSError:
                continue
            sys.stdout.write("/" + os.path.relpath(p, root) + "\0")
PY
N=$(tr -cd '\0' < "$LIST" | wc -c)
# one file per ldd call, so that every "not found" line has its file name next to it
chroot "$ROOT" /usr/bin/xargs -0 -n 1 /bin/sh -c 'ldd "$1" 2>/dev/null | sed "s#^#$1: #"' sh < "$LIST" > "$OUT" || true
if grep -q 'not found' "$OUT"; then
  echo "check-libs.sh: UNRESOLVED libraries:" >&2; grep 'not found' "$OUT" >&2; exit 1
fi
echo "check-libs.sh: $N ELF files checked, no unresolved library"
