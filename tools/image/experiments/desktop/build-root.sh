#!/bin/bash
# build-root.sh OUT.sqsh: the desktop experiment root = the desktop base (packages of desktop.list) + image/rootfs +
# the experiment overlay + driftwm + hubd + efibootmgr. No PAM/apt strip (the experiment is about graphics).
# Needs $WORK/base (build-base.sh with MACHINE=desktop.build) and $WORK/out/driftwm (build-driftwm.sh). EXPERIMENT.
. "$(dirname "$0")/../../common.sh"
load_machine
OUT=${1:?usage: build-root.sh OUT.sqsh}
HERE=$(cd "$(dirname "$0")" && pwd)
[ -d "$WORK/base" ] && [ -x "$WORK/out/driftwm" ] || { echo "run build-base.sh and build-driftwm.sh first" >&2; exit 2; }
[ -x "$WORK/out/hubd" ] || ( cd "$REPO" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o "$WORK/out/hubd" ./cmd/hubd )
S=$WORK/stage-desktop
rm -rf "$S"; cp -a "$WORK/base" "$S"
cp -a "$REPO/image/rootfs/." "$S/"
cp -a "$HERE/overlay/." "$S/"
strip -s -o "$S/usr/local/bin/driftwm" "$WORK/out/driftwm"; chmod 0755 "$S/usr/local/bin/driftwm"
install -m 0755 "$WORK/out/hubd" "$S/usr/bin/hubd"
[ -x "$WORK/out/fakenode" ] || ( cd "$REPO" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o "$WORK/out/fakenode" ./tools/fakenode )
install -m 0755 "$WORK/out/fakenode" "$S/usr/bin/fakenode"
install -m 0755 "$T/bin/efibootmgr" "$S/usr/sbin/efibootmgr"
cp -a "$T"/usr/lib/x86_64-linux-gnu/libefivar.so.1* "$T"/usr/lib/x86_64-linux-gnu/libefiboot.so.1* "$S/usr/lib/x86_64-linux-gnu/"
for a in ip udhcpc wget cttyhack reboot halt poweroff hostname stat awk head sha256sum getty ps pidof; do ln -sf /bin/busybox "$S/usr/local/bin/$a"; done
rm -rf "$S/etc/systemd" "$S/usr/lib/systemd" "$S/var/lib/systemd" "$S/usr/lib/udev" "$S/etc/udev"
# eudev (built by build-eudev.sh) replaces the udev database workaround: udevd, udevadm, the rules and eudev's own
# libudev.so.1 (put where the dynamic loader finds it first, over the Ubuntu package's libudev from the systemd sources).
if [ -d "$WORK/out/eudev-root/usr" ]; then
  cp -a "$WORK/out/eudev-root/." "$S/"
  rm -f "$S"/usr/lib/x86_64-linux-gnu/libudev.so.1*
  cp -a "$S"/usr/lib/libudev.so.1.6.3 "$S/usr/lib/x86_64-linux-gnu/libudev.so.1.6.3"
  ln -sf libudev.so.1.6.3 "$S/usr/lib/x86_64-linux-gnu/libudev.so.1"
  rm -f "$S"/usr/lib/libudev.so* "$S"/usr/lib/libudev.la; rm -rf "$S/usr/include/libudev.h" "$S/usr/lib/pkgconfig"
fi
# the normal user "hub" (uid 1000) and the group "seat", for the seat daemon
grep -q '^hub:' "$S/etc/passwd" || echo 'hub:x:1000:1000:hub:/run/hub:/bin/sh' >> "$S/etc/passwd"
grep -q '^hub:' "$S/etc/group" || echo 'hub:x:1000:' >> "$S/etc/group"
grep -q '^seat:' "$S/etc/group" || echo 'seat:x:1001:hub' >> "$S/etc/group"
for g in input video render; do grep -q "^$g:" "$S/etc/group" || echo "$g:x:$((1100 + $(echo $g | cksum | cut -c1-2))):" >> "$S/etc/group"; done
mkdir -p "$S/etc/hubos" "$S/config" "$S/data" "$S/boot/efi" "$S/run" "$S/tmp" "$S/var" "$S/root"
printf 'version=1\nflavor=desktop-experiment\nkernel-version=%s\n' "$KERNEL_VERSION" > "$S/etc/hubos-release"
mksquashfs "$S" "$OUT" -comp zstd -quiet -noappend -all-root >/dev/null
rm -rf "$S"
say "root image $OUT: $(stat -c %s "$OUT") bytes"
