#!/bin/bash
# build-hub-parts.sh: builds the two programs the hub image needs that no package gives us, inside a throw-away
# build root (mmdebstrap from the same Ubuntu snapshot as the image, so they link the image's own libraries):
#   driftwm  at the PINNED commit (HUB-OS.md), a release build; the host's Rust toolchain is bind-mounted into the
#            build root (the snapshot's rustc is too old)                   -> $WORK/out/driftwm
#   eudev    PINNED release 3.2.14 (tarball hash below), a udev for systems without systemd, GPL-2.0-or-later
#            (libudev: LGPL-2.1-or-later)                                    -> $WORK/out/eudev-root (an install tree)
# Needs root, network (github.com, crates.io, the Ubuntu snapshot). Nothing is installed on the host. Results are kept
# in $WORK/out between runs.
. "$(dirname "$0")/common.sh"
load_machine
DRIFTWM_COMMIT=352333a8fa1b22171492d4b71a54102045c9a19d
EUDEV_VER=3.2.14
EUDEV_SHA256=8da4319102f24abbf7fff5ce9c416af848df163b29590e666d334cc1927f006f
BR=$WORK/hub-buildroot
if [ -x "$WORK/out/driftwm" ] && [ -d "$WORK/out/eudev-root/usr" ]; then say "driftwm and eudev already built"; exit 0; fi
if [ ! -d "$BR/usr" ]; then
  say "build root (dev packages)"
  rm -rf "$BR"; mkdir -p "$BR"
  mmdebstrap --mode=root --variant=minbase --components=main,universe \
    --include=build-essential,pkg-config,git,ca-certificates,clang,libclang-dev,cmake,libwayland-dev,libxkbcommon-dev,libinput-dev,libudev-dev,libseat-dev,libgbm-dev,libegl-dev,libgles-dev,libdrm-dev,libdisplay-info-dev,libpixman-1-dev,libfontconfig-dev,libfreetype-dev,gperf \
    noble "$WORK/out/buildroot.tar" "http://snapshot.ubuntu.com/ubuntu/$SNAPSHOT" >/dev/null
  tar -C "$BR" --numeric-owner -xpf "$WORK/out/buildroot.tar"
fi
[ -f /root/.ccr/ca-bundle.crt ] && { mkdir -p "$BR/etc/ssl/certs"; cp /root/.ccr/ca-bundle.crt "$BR/etc/ssl/certs/ca-bundle-proxy.crt"; }
cp /etc/resolv.conf "$BR/etc/resolv.conf" 2>/dev/null || true
CARGO_HOME_HOST=${CARGO_HOME:-$HOME/.cargo}; RUSTUP_HOST=${RUSTUP_HOME:-$HOME/.rustup}
cleanup() { umount "$BR/dev" "$BR/proc" "$BR/out" "$BR/src" "$BR/eudev-src" "$BR/host/rustup" "$BR/host/cargo" 2>/dev/null || true; }
trap cleanup EXIT
mkdir -p "$BR/host/cargo" "$BR/host/rustup" "$BR/src" "$BR/eudev-src" "$BR/out" "$BR/proc" "$BR/dev"
mount -t proc proc "$BR/proc"; mount --bind /dev "$BR/dev"; mount --bind "$WORK/out" "$BR/out"

# ---- driftwm ----
if [ ! -x "$WORK/out/driftwm" ]; then
  SRC=$WORK/driftwm-src
  if [ ! -d "$SRC/.git" ]; then
    say "cloning driftwm"
    GIT_LFS_SKIP_SMUDGE=1 git clone -q --depth 50 https://github.com/malbiruk/driftwm "$SRC"
  fi
  git -C "$SRC" checkout -q "$DRIFTWM_COMMIT" || { git -C "$SRC" fetch -q --depth 500 origin "$DRIFTWM_COMMIT" && git -C "$SRC" checkout -q "$DRIFTWM_COMMIT"; }
  [ "$(git -C "$SRC" rev-parse HEAD)" = "$DRIFTWM_COMMIT" ] || { echo "driftwm is not at the pinned commit" >&2; exit 1; }
  say "driftwm at $DRIFTWM_COMMIT"
  mount --bind "$CARGO_HOME_HOST" "$BR/host/cargo"; mount --bind "$RUSTUP_HOST" "$BR/host/rustup"; mount --bind "$SRC" "$BR/src"
  s=$(date +%s)
  chroot "$BR" /usr/bin/env -i HOME=/root PATH=/host/cargo/bin:/usr/sbin:/usr/bin:/sbin:/bin CARGO_HOME=/host/cargo RUSTUP_HOME=/host/rustup \
    CARGO_NET_GIT_FETCH_WITH_CLI=true HTTPS_PROXY="${HTTPS_PROXY:-}" HTTP_PROXY="${HTTP_PROXY:-}" NO_PROXY="${NO_PROXY:-}" \
    SSL_CERT_FILE=/etc/ssl/certs/ca-bundle-proxy.crt GIT_SSL_CAINFO=/etc/ssl/certs/ca-bundle-proxy.crt CARGO_HTTP_CAINFO=/etc/ssl/certs/ca-bundle-proxy.crt \
    CARGO_TARGET_DIR=/out/driftwm-target SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" \
    /bin/sh -c 'cd /src && cargo build --release 2>&1' > "$WORK/out/driftwm-build.log" || { tail -n 30 "$WORK/out/driftwm-build.log" >&2; exit 1; }
  cp "$WORK/out/driftwm-target/release/driftwm" "$WORK/out/driftwm"
  umount "$BR/src" "$BR/host/rustup" "$BR/host/cargo"
  say "driftwm built in $(( $(date +%s) - s )) s: $(stat -c %s "$WORK/out/driftwm") bytes"
fi

# ---- eudev ----
if [ ! -d "$WORK/out/eudev-root/usr" ]; then
  ESRC=$WORK/eudev-$EUDEV_VER
  if [ ! -d "$ESRC" ]; then
    say "downloading eudev $EUDEV_VER"
    curl -sSL -o "$WORK/eudev-$EUDEV_VER.tar.gz" "https://github.com/eudev-project/eudev/releases/download/v$EUDEV_VER/eudev-$EUDEV_VER.tar.gz"
    [ "$(sha256sum "$WORK/eudev-$EUDEV_VER.tar.gz" | cut -d' ' -f1)" = "$EUDEV_SHA256" ] || { echo "eudev tarball: sha256 does not match the pinned value" >&2; exit 1; }
    tar -C "$WORK" -xzf "$WORK/eudev-$EUDEV_VER.tar.gz"
  fi
  mount --bind "$ESRC" "$BR/eudev-src"
  s=$(date +%s)
  chroot "$BR" /usr/bin/env -i HOME=/root PATH=/usr/sbin:/usr/bin:/sbin:/bin SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" /bin/sh -c '
    set -e
    rm -rf /tmp/eudev-build && cp -a /eudev-src /tmp/eudev-build && cd /tmp/eudev-build
    ./configure --prefix=/usr --sysconfdir=/etc --disable-manpages --disable-kmod --disable-blkid --disable-selinux --disable-introspection --disable-hwdb --disable-static > /out/eudev-configure.log 2>&1
    make -j4 > /out/eudev-make.log 2>&1
    rm -rf /out/eudev-root && make DESTDIR=/out/eudev-root install > /out/eudev-install.log 2>&1
  ' || { tail -n 30 "$WORK/out/eudev-configure.log" "$WORK/out/eudev-make.log" 2>/dev/null >&2; exit 1; }
  umount "$BR/eudev-src"
  say "eudev $EUDEV_VER built in $(( $(date +%s) - s )) s; files: $(cd "$WORK/out/eudev-root" && find . -type f | wc -l)"
fi
