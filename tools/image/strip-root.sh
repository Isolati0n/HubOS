#!/bin/bash
# strip-root.sh ROOT: delete the files of the packages a finished image does not need: apt, libapt-pkg6.0t64,
# procps, libproc2-0, libpam-runtime, libpam-modules, libpam-modules-bin, login, passwd (docs/proposals/
# systemd-libraries.md section 4.2, step A). Nodes are never changed with a package manager. It does NOT delete
# the two systemd libraries (libsystemd0, libudev1) or the programs that link them (step B is not done).
# The files come from dpkg's own lists (ROOT/var/lib/dpkg/info/PACKAGE.list); directories are left alone.
set -e
ROOT=$(cd "${1:?usage: strip-root.sh ROOT}" && pwd)
PKGS="apt libapt-pkg6.0t64 procps libproc2-0 libpam-runtime libpam-modules libpam-modules-bin login passwd"
n=0
for p in $PKGS; do
  l=$ROOT/var/lib/dpkg/info/$p.list
  [ -f "$l" ] || l=$(ls "$ROOT"/var/lib/dpkg/info/"$p":*.list 2>/dev/null | head -n 1)   # multi-arch packages: PACKAGE:amd64.list
  # mmdebstrap's minbase root has no apt and no libapt-pkg6.0t64; a package that is not installed has no list
  if [ -z "$l" ] || [ ! -f "$l" ]; then
    if grep -q "^Package: $p\$" "$ROOT/var/lib/dpkg/status"; then echo "strip-root.sh: $p is installed but has no file list" >&2; exit 1; fi
    echo "strip-root.sh: $p is not installed in this root (nothing to remove)"; continue
  fi
  while IFS= read -r f; do
    t=$ROOT$f
    if [ -e "$t" ] || [ -L "$t" ]; then
      if [ ! -d "$t" ] || [ -L "$t" ]; then rm -f "$t"; n=$((n+1)); fi
    fi
  done < "$l"
done
echo "strip-root.sh: removed $n files of: $PKGS"
