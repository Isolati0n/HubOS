#!/bin/bash
# build-root-image.sh VERSION FLAVOR OUT.sqsh PUBKEY
#   Makes the squashfs root of one release from $WORK/base, the files in image/rootfs, the static hubd
#   built from this repo, efibootmgr and signify. PUBKEY is the (public) update key built into the
#   image. FLAVOR: good | noinit | unhealthy | hang | garbage (the last four are bad test images).
. "$(dirname "$0")/common.sh"
load_machine
V=$1; F=$2; OUT=$3; PUB=$4
[ -n "$PUB" ] && [ -f "$PUB" ] || { echo "usage: build-root-image.sh VERSION FLAVOR OUT.sqsh PUBKEY" >&2; exit 2; }
[ -d "$WORK/base" ] || { echo "run build-base.sh first" >&2; exit 2; }
if [ "$F" = garbage ]; then head -c 2097152 /dev/zero > "$OUT"; exit 0; fi
if [ ! -x "$WORK/out/hubd" ]; then
  say "building hubd (static)"
  ( cd "$REPO" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o "$WORK/out/hubd" ./cmd/hubd )
fi
S=$WORK/stage-$V-$F
rm -rf "$S"; cp -a "$WORK/base" "$S"
cp -a "$REPO/image/rootfs/." "$S/"
install -m 0755 "$WORK/out/hubd" "$S/usr/bin/hubd"
install -m 0755 "$T/bin/efibootmgr" "$S/usr/sbin/efibootmgr"
cp -a "$T"/usr/lib/x86_64-linux-gnu/libefivar.so.1* "$T"/usr/lib/x86_64-linux-gnu/libefiboot.so.1* "$S/usr/lib/x86_64-linux-gnu/"
install -m 0755 "$T/bin/signify-openbsd" "$S/usr/bin/signify-openbsd"
cp -a "$T"/usr/lib/x86_64-linux-gnu/libbsd.so.0* "$S/usr/lib/x86_64-linux-gnu/"
# Unit files and rules that packages ship for systemd and udev (e2fsprogs, util-linux, dpkg): inert
# without systemd, and an s6 image has no use for them. The two libraries stay (docs/image.md).
rm -rf "$S/etc/systemd" "$S/usr/lib/systemd" "$S/var/lib/systemd" "$S/usr/lib/udev" "$S/etc/udev"
mkdir -p "$S/etc/hubos" "$S/usr/local/bin" "$S/config" "$S/data" "$S/boot/efi" "$S/run" "$S/tmp" "$S/var"
install -m 0644 "$PUB" "$S/etc/hubos/update.pub"
for a in ip udhcpc wget cttyhack reboot halt poweroff hostname stat awk head sha256sum getty watchdog; do ln -sf /bin/busybox "$S/usr/local/bin/$a"; done
printf 'version=%s\nflavor=%s\nkernel-version=%s\n' "$V" "$F" "$KERNEL_VERSION" > "$S/etc/hubos-release"
{ echo 'root::0:0:root:/root:/bin/sh'; grep -v '^root:' "$S/etc/passwd"; echo 'hub:x:1000:1000:hub:/run/hubos:/bin/false'; } > "$S/etc/passwd.new"
mv "$S/etc/passwd.new" "$S/etc/passwd"; echo 'hub:x:1000:' >> "$S/etc/group"
case $F in
  noinit)    rm -f "$S/usr/sbin/init" ;;
  unhealthy) printf '#!/bin/sh\nexec sleep 100000\n' > "$S/etc/s6/sv/hubd/run" ;;
  hang)      printf '#!/bin/sh\nexec sleep 100000\n' > "$S/usr/sbin/init" ;;
  good) ;;
  *) echo "unknown flavor $F" >&2; exit 2 ;;
esac
mksquashfs "$S" "$OUT" -comp zstd -quiet -noappend -all-root >/dev/null
rm -rf "$S"
