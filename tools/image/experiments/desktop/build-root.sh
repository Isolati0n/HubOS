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
install -m 0755 "$WORK/out/driftwm" "$S/usr/local/bin/driftwm"
install -m 0755 "$WORK/out/hubd" "$S/usr/bin/hubd"
install -m 0755 "$T/bin/efibootmgr" "$S/usr/sbin/efibootmgr"
cp -a "$T"/usr/lib/x86_64-linux-gnu/libefivar.so.1* "$T"/usr/lib/x86_64-linux-gnu/libefiboot.so.1* "$S/usr/lib/x86_64-linux-gnu/"
for a in ip udhcpc wget cttyhack reboot halt poweroff hostname stat awk head sha256sum getty ps pidof; do ln -sf /bin/busybox "$S/usr/local/bin/$a"; done
rm -rf "$S/etc/systemd" "$S/usr/lib/systemd" "$S/var/lib/systemd" "$S/usr/lib/udev" "$S/etc/udev"
mkdir -p "$S/etc/hubos" "$S/config" "$S/data" "$S/boot/efi" "$S/run" "$S/tmp" "$S/var" "$S/root"
printf 'version=1\nflavor=desktop-experiment\nkernel-version=%s\n' "$KERNEL_VERSION" > "$S/etc/hubos-release"
mksquashfs "$S" "$OUT" -comp zstd -quiet -noappend -all-root >/dev/null
rm -rf "$S"
say "root image $OUT: $(stat -c %s "$OUT") bytes"
