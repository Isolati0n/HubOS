#!/bin/bash
# build-eudev.sh: builds eudev (a udev for systems without systemd) from a pinned source release inside the throw-away
# build root of build-driftwm.sh, and leaves the result in $WORK/out/eudev-root (an install tree: usr/, etc/). Nothing is
# installed on the host. Needs the build root (run build-driftwm.sh first), network (github.com), root. EXPERIMENT.
. "$(dirname "$0")/../../common.sh"
load_machine
VER=3.2.14
URL=https://github.com/eudev-project/eudev/releases/download/v$VER/eudev-$VER.tar.gz
BR=$WORK/driftwm-buildroot
[ -d "$BR/usr" ] || { echo "run build-driftwm.sh first (it makes the build root)" >&2; exit 2; }
[ -d "$WORK/out/eudev-root/usr" ] && { say "eudev already built"; exit 0; }
SRC=$WORK/eudev-$VER
if [ ! -d "$SRC" ]; then
  say "downloading eudev $VER"
  curl -sSL -o "$WORK/eudev-$VER.tar.gz" "$URL"
  say "sha256 of the downloaded tarball: $(sha256sum "$WORK/eudev-$VER.tar.gz" | cut -d' ' -f1)"
  [ -z "$EUDEV_SHA256" ] || [ "$(sha256sum "$WORK/eudev-$VER.tar.gz" | cut -d' ' -f1)" = "$EUDEV_SHA256" ] || { echo "eudev tarball hash mismatch" >&2; exit 1; }
  tar -C "$WORK" -xzf "$WORK/eudev-$VER.tar.gz"
fi
mkdir -p "$BR/eudev-src" "$BR/eudev-out" "$BR/proc" "$BR/dev"
mount --bind "$SRC" "$BR/eudev-src"; mount --bind "$WORK/out" "$BR/eudev-out"; mount -t proc proc "$BR/proc"; mount --bind /dev "$BR/dev"
trap 'umount "$BR/dev" "$BR/proc" "$BR/eudev-out" "$BR/eudev-src" 2>/dev/null' EXIT
cp /etc/resolv.conf "$BR/etc/resolv.conf" 2>/dev/null || true
s=$(date +%s)
chroot "$BR" /usr/bin/env -i HOME=/root PATH=/usr/sbin:/usr/bin:/sbin:/bin SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" /bin/sh -c '
  set -e
  rm -rf /tmp/eudev-build && cp -a /eudev-src /tmp/eudev-build && cd /tmp/eudev-build
  ./configure --prefix=/usr --sysconfdir=/etc --disable-manpages --disable-kmod --disable-blkid --disable-selinux --disable-introspection --disable-hwdb --disable-static > /eudev-out/eudev-configure.log 2>&1
  make -j4 > /eudev-out/eudev-make.log 2>&1
  rm -rf /eudev-out/eudev-root && make DESTDIR=/eudev-out/eudev-root install > /eudev-out/eudev-install.log 2>&1
' || { tail -n 30 "$WORK/out/eudev-configure.log" "$WORK/out/eudev-make.log" 2>/dev/null >&2; exit 1; }
say "eudev $VER built in $(( $(date +%s) - s )) s; files: $(cd "$WORK/out/eudev-root" && find . -type f | wc -l)"
