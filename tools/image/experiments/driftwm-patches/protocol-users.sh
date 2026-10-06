#!/bin/bash
# protocol-users.sh ROOT : for each Wayland interface that the hub's compositor hides (patch 0020-d8), list the programs and
# libraries in ROOT (an unpacked hub image root, for example `unsquashfs -d ROOT rootfs.sqsh`) that contain the interface's
# name. A client can only bind an interface whose name is in its own code (the name comes from the protocol code that is
# compiled into the client or one of its libraries), so "no file" is evidence that nothing on the image can ask for it.
# "Some file" is NOT proof that it is used: Waybar, for example, has a taskbar module that would use wlr-foreign-toplevel but
# the hub's bar does not load it (etc/hubos/waybar.json); the run-time evidence (H1b of the hub image test) says what each
# client really binds.
ulimit -c 0
ROOT=${1:?usage: protocol-users.sh ROOT}
HERE=$(cd "$(dirname "$0")" && pwd)
LIST=$(sed -n '/^var hiddenProtocols/,/^}/p' "$HERE/../../hub_restore_test.go" | grep -o '"[a-z_0-9]*"' | tr -d '"')
for i in $LIST; do
  files=$(grep -rlaF --exclude=driftwm --exclude-dir=proc --exclude-dir=sys -e "$i" "$ROOT/usr/bin" "$ROOT/usr/sbin" "$ROOT/usr/local/bin" "$ROOT/usr/lib" 2>/dev/null | sed "s#^$ROOT##" | sort | tr '\n' ' ')
  echo "$i: ${files:-(no file)}"
done
