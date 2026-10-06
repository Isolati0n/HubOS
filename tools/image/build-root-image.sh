#!/bin/bash
# build-root-image.sh VERSION FLAVOR OUT.sqsh PUBKEY
#   Makes the squashfs root of one release from $WORK/base, the files in image/rootfs, the static hubd
#   built from this repo, efibootmgr, signify and the recovery kernel. PUBKEY is the (public) update key built into the
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
[ -n "$ROOTFS_OVERLAY" ] && cp -a "$REPO/$ROOTFS_OVERLAY/." "$S/"
install -m 0755 "$WORK/out/hubd" "$S/usr/bin/hubd"
install -m 0755 "$T/bin/efibootmgr" "$S/usr/sbin/efibootmgr"
cp -a "$T"/usr/lib/x86_64-linux-gnu/libefivar.so.1* "$T"/usr/lib/x86_64-linux-gnu/libefiboot.so.1* "$S/usr/lib/x86_64-linux-gnu/"
install -m 0755 "$T/bin/signify-openbsd" "$S/usr/bin/signify-openbsd"
cp -a "$T"/usr/lib/x86_64-linux-gnu/libbsd.so.0* "$S/usr/lib/x86_64-linux-gnu/"
# Unit files and rules that packages ship for systemd and udev (e2fsprogs, util-linux, dpkg): inert
# without systemd, and an s6 image has no use for them. The two libraries stay (docs/image.md).
rm -rf "$S/etc/systemd" "$S/usr/lib/systemd" "$S/var/lib/systemd" "$S/usr/lib/udev" "$S/etc/udev"
if [ "$EXTRA_PARTS" = hub ]; then
  # The hub: driftwm and eudev, built from source at pinned versions (build-hub-parts.sh). eudev's libudev.so.1 replaces
  # the one from the systemd sources (the package's files are overwritten); udevd, udevadm and the rules come with it.
  "$(dirname "$0")/build-hub-parts.sh" >&2
  install -m 0755 -D "$WORK/out/driftwm" "$S/usr/local/bin/driftwm"; strip -s "$S/usr/local/bin/driftwm"
  cp -a "$WORK/out/eudev-root/." "$S/"
  rm -f "$S"/usr/lib/x86_64-linux-gnu/libudev.so.1*
  cp -a "$S/usr/lib/libudev.so.1.6.3" "$S/usr/lib/x86_64-linux-gnu/libudev.so.1.6.3"
  ln -sf libudev.so.1.6.3 "$S/usr/lib/x86_64-linux-gnu/libudev.so.1"
  rm -f "$S"/usr/lib/libudev.so* "$S"/usr/lib/libudev.la; rm -rf "$S/usr/include/libudev.h" "$S/usr/lib/pkgconfig"
  echo 'seat:x:1001:hub' >> "$S/etc/group"
  for g in input video render; do grep -q "^$g:" "$S/etc/group" || echo "$g:x:$((1100 + $(echo $g | cksum | cut -c1-2))):" >> "$S/etc/group"; done
  # The compositor's settings are baked into the image, so a mistake in them stops the BUILD here (an unknown field,
  # a bad value, restore_windows not written as false): see check-driftwm-config.sh.
  "$(dirname "$0")/check-driftwm-config.sh" "$S" /etc/hubos/driftwm.toml >&2 || { echo "the hub's driftwm settings file is not acceptable; no image was built" >&2; exit 1; }
fi
mkdir -p "$S/etc/hubos" "$S/usr/local/bin" "$S/config" "$S/data" "$S/boot/efi" "$S/run" "$S/tmp" "$S/var"
# The update keyring: every *.pub in /etc/hubos/keys is accepted by the update tool and by the recovery kernel (a release
# signed with the old key can carry the next key; the old key is dropped in a later release, docs/image.md).
# KEYRING_PUBS (space-separated public key files) overrides the default, the single PUBKEY argument.
mkdir -p "$S/etc/hubos/keys"
for k in ${KEYRING_PUBS:-$PUB}; do install -m 0644 "$k" "$S/etc/hubos/keys/$(sha256sum "$k" | cut -c1-16).pub"; done
# The recovery kernel travels inside the root (the confirm step installs it after a healthy boot; the manifest lists
# its hash). RECOVERY_KERNEL_FILE and RECOVERY_VERSION_OVERRIDE let a test build a root with another recovery kernel;
# NO_RECOVERY=1 builds a root without one.
if [ -z "$NO_RECOVERY" ]; then
  mkdir -p "$S/usr/lib/hubos/recovery"
  install -m 0644 "${RECOVERY_KERNEL_FILE:-$WORK/out/kernel-recovery.efi}" "$S/usr/lib/hubos/recovery/kernel-recovery.efi"
  echo "${RECOVERY_VERSION_OVERRIDE:-${RECOVERY_VERSION:-1}}" > "$S/usr/lib/hubos/recovery/recovery.version"
fi
for a in ip udhcpc wget cttyhack reboot halt poweroff hostname stat awk head sha256sum getty watchdog ps pidof; do ln -sf /bin/busybox "$S/usr/local/bin/$a"; done
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
# A finished image has no package manager, no PAM modules and no procps (busybox supplies ps and pidof above);
# the build fails if any file left in /usr, /bin, /sbin or /lib has a library that cannot be found.
"$(dirname "$0")/strip-root.sh" "$S" >&2
"$(dirname "$0")/check-libs.sh" "$S" >&2
mksquashfs "$S" "$OUT" -comp zstd -quiet -noappend -all-root >/dev/null
rm -rf "$S"
