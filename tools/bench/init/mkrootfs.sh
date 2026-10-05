#!/bin/bash
# mkrootfs.sh CAND : build $W/rf/CAND.cpio.gz for the benchmark candidate (BENCH CODE, not for the image).
# Needs the tools unpacked as in README.md (env.sh sets T, W). Nothing is installed on the machine.
set -e
HERE=$(cd "$(dirname "$0")" && pwd)
CAND=$1; : "${W:?}" "${T:?}"
R=$W/rf/$CAND; rm -rf "$R"; mkdir -p "$R"/{bin,sbin,usr/bin,usr/sbin,lib,lib64,dev,proc,sys,run,var,tmp,etc,opt/s6/bin,lib/modules,cand,ops,root}
cp "$T/usr/bin/busybox" "$R/bin/busybox"
for a in $("$R/bin/busybox" --list); do [ -e "$R/bin/$a" ] || ln -s busybox "$R/bin/$a"; done
cp "$W/hubsim" "$R/bin/hubsim"
for s in seatd udevd dbus driftwm waybar hubd; do ln -s hubsim "$R/bin/$s"; done
unzstd -q -c "$W"/modroot/lib/modules/*/kernel/drivers/watchdog/i6300esb.ko.zst > "$R/lib/modules/i6300esb.ko"
cp "$HERE/common/init.sh" "$R/init"; cp "$HERE/common/policy" "$R/bin/policy"; chmod +x "$R/init" "$R/bin/policy"
# addbin SRC [DEST]: copy a program and the shared libraries it needs
addbin() { local src=$1 dst=${2:-$1}; mkdir -p "$R$(dirname "$dst")"; cp "$src" "$R$dst"
  for l in $(ldd "$src" 2>/dev/null | grep -oE '/[^ ]+' ); do mkdir -p "$R$(dirname "$l")"; cp -n "$l" "$R$l" 2>/dev/null || true; done; }
S6BINS="s6-svscan s6-supervise s6-svc s6-svstat s6-svok s6-svscanctl s6-svwait s6-log s6-rc s6-rc-init s6-rc-compile s6-rc-db s6-notifyoncheck s6-setuidgid s6-envdir s6-sleep s6-ftrig-wait s6-ftrig-notify s6-fghack s6-ps s6-rc-update"
copy_s6() { for b in $S6BINS; do [ -x "$W/skel/bin/$b" ] && cp "$W/skel/bin/$b" "$R/opt/s6/bin/$b"; done
  for b in execlineb exec foreground background if ifelse fdmove redirfd cd importas define elgetpositionals; do [ -x "$W/skel/bin/$b" ] && cp "$W/skel/bin/$b" "$R/opt/s6/bin/$b"; done; return 0; }
. "$HERE/candidates/$CAND/build.sh"
[ -d "$HERE/candidates/$CAND/rootfs" ] && cp -a "$HERE/candidates/$CAND/rootfs/." "$R/"
chmod -R u+rwX "$R"
( cd "$R" && find . | cpio -o -H newc -R 0:0 --quiet | gzip -1 > "$W/rf/$CAND.cpio.gz" )
ls -l "$W/rf/$CAND.cpio.gz" | awk '{print "initramfs bytes:", $5}'
