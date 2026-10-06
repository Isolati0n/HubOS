#!/bin/bash
# build-hub-parts.sh: builds the two programs the hub image needs that no package gives us, inside a throw-away
# build root (mmdebstrap from the same Ubuntu snapshot as the image, so they link the image's own libraries):
#   driftwm  at the PINNED commit (HUB-OS.md), with the Hub OS patch set (image/patches/driftwm/: driftwm patches
#            0*.patch and Smithay patches smithay/0*.patch, both against the pins in image/patches/driftwm/PINS),
#            a release build; the host's Rust toolchain is bind-mounted into the build root (the snapshot's rustc is
#            too old). The build FAILS if a patch does not apply or if the pins disagree        -> $WORK/out/driftwm
#   eudev    PINNED release 3.2.14 (tarball hash below), a udev for systems without systemd, GPL-2.0-or-later
#            (libudev: LGPL-2.1-or-later)                                    -> $WORK/out/eudev-root (an install tree)
# Needs root, network (github.com, crates.io, the Ubuntu snapshot). Nothing is installed on the host. Results are kept
# in $WORK/out between runs.
. "$(dirname "$0")/common.sh"
load_machine
PATCHES=$REPO/image/patches/driftwm
. "$PATCHES/PINS"   # DRIFTWM_COMMIT and SMITHAY_REV
# A change to a patch or to a pin rebuilds driftwm (the stamp is kept next to the binary).
PATCH_STAMP=$(cat "$PATCHES/PINS" "$PATCHES"/0*.patch "$PATCHES"/smithay/0*.patch | sha256sum | cut -d' ' -f1)
driftwm_current() { [ -x "$WORK/out/driftwm" ] && [ "$(cat "$WORK/out/driftwm.stamp" 2>/dev/null)" = "$PATCH_STAMP" ]; }
EUDEV_VER=3.2.14
EUDEV_SHA256=8da4319102f24abbf7fff5ce9c416af848df163b29590e666d334cc1927f006f
BR=$WORK/hub-buildroot
if driftwm_current && [ -d "$WORK/out/eudev-root/usr" ]; then say "driftwm and eudev already built"; exit 0; fi
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
cleanup() { umount "$BR/dev" "$BR/proc" "$BR/out" "$BR/src" "$BR/smithay" "$BR/eudev-src" "$BR/host/rustup" "$BR/host/cargo" 2>/dev/null || true; }
trap cleanup EXIT
mkdir -p "$BR/host/cargo" "$BR/host/rustup" "$BR/src" "$BR/smithay" "$BR/eudev-src" "$BR/out" "$BR/proc" "$BR/dev"
mount -t proc proc "$BR/proc"; mount --bind /dev "$BR/dev"; mount --bind "$WORK/out" "$BR/out"

# ---- driftwm ----
if ! driftwm_current; then
  SRC=$WORK/driftwm-src
  if [ ! -d "$SRC/.git" ]; then
    say "cloning driftwm"
    GIT_LFS_SKIP_SMUDGE=1 git clone -q --depth 50 https://github.com/malbiruk/driftwm "$SRC"
  fi
  git -C "$SRC" checkout -q -f "$DRIFTWM_COMMIT" || { git -C "$SRC" fetch -q --depth 500 origin "$DRIFTWM_COMMIT" && git -C "$SRC" checkout -q -f "$DRIFTWM_COMMIT"; }
  # back to the pristine pinned tree (an earlier build left patched files in it)
  git -C "$SRC" reset -q --hard "$DRIFTWM_COMMIT"; git -C "$SRC" clean -q -fdx
  [ "$(git -C "$SRC" rev-parse HEAD)" = "$DRIFTWM_COMMIT" ] || { echo "driftwm is not at the pinned commit" >&2; exit 1; }
  say "driftwm at $DRIFTWM_COMMIT"
  # Smithay: the revision that driftwm's Cargo.lock pins must be the one the Smithay patches are written against.
  grep -q "git+https://github.com/Smithay/smithay.git?rev=$SMITHAY_REV#$SMITHAY_REV" "$SRC/Cargo.lock" \
    || { echo "driftwm's Cargo.lock does not pin Smithay $SMITHAY_REV (see image/patches/driftwm/PINS)" >&2; exit 1; }
  SM=$WORK/smithay-src
  if [ ! -d "$SM/.git" ]; then
    say "cloning Smithay"
    git clone -q --no-checkout https://github.com/Smithay/smithay.git "$SM"
  fi
  git -C "$SM" checkout -q -f "$SMITHAY_REV" 2>/dev/null || { git -C "$SM" fetch -q origin "$SMITHAY_REV" && git -C "$SM" checkout -q -f "$SMITHAY_REV"; }
  git -C "$SM" reset -q --hard "$SMITHAY_REV"; git -C "$SM" clean -q -fdx
  [ "$(git -C "$SM" rev-parse HEAD)" = "$SMITHAY_REV" ] || { echo "Smithay is not at the pinned revision" >&2; exit 1; }
  # Apply the patch set. git apply is strict (no fuzz): a patch that does not apply stops the build.
  for p in "$PATCHES"/0*.patch; do
    git -C "$SRC" apply --check "$p" && git -C "$SRC" apply "$p" || { echo "driftwm patch $(basename "$p") does not apply to $DRIFTWM_COMMIT" >&2; exit 1; }
    say "applied driftwm patch $(basename "$p")"
  done
  for p in "$PATCHES"/smithay/0*.patch; do
    git -C "$SM" apply --check "$p" && git -C "$SM" apply "$p" || { echo "Smithay patch $(basename "$p") does not apply to $SMITHAY_REV" >&2; exit 1; }
    say "applied Smithay patch $(basename "$p")"
  done
  # driftwm builds against the patched Smithay copy (the same source URL as in its Cargo.toml, replaced by a path).
  cat >> "$SRC/Cargo.toml" <<'TOML'

[patch."https://github.com/Smithay/smithay.git"]
smithay = { path = "/smithay" }
smithay-drm-extras = { path = "/smithay/smithay-drm-extras" }
TOML
  rm -rf "$WORK/out/driftwm-target" "$WORK/out/driftwm" "$WORK/out/driftwm.stamp"
  mount --bind "$CARGO_HOME_HOST" "$BR/host/cargo"; mount --bind "$RUSTUP_HOST" "$BR/host/rustup"; mount --bind "$SRC" "$BR/src"; mount --bind "$SM" "$BR/smithay"
  s=$(date +%s)
  chroot "$BR" /usr/bin/env -i HOME=/root PATH=/host/cargo/bin:/usr/sbin:/usr/bin:/sbin:/bin CARGO_HOME=/host/cargo RUSTUP_HOME=/host/rustup \
    CARGO_NET_GIT_FETCH_WITH_CLI=true HTTPS_PROXY="${HTTPS_PROXY:-}" HTTP_PROXY="${HTTP_PROXY:-}" NO_PROXY="${NO_PROXY:-}" \
    SSL_CERT_FILE=/etc/ssl/certs/ca-bundle-proxy.crt GIT_SSL_CAINFO=/etc/ssl/certs/ca-bundle-proxy.crt CARGO_HTTP_CAINFO=/etc/ssl/certs/ca-bundle-proxy.crt \
    CARGO_TARGET_DIR=/out/driftwm-target SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" \
    /bin/sh -c 'cd /src && cargo build --release 2>&1' > "$WORK/out/driftwm-build.log" || { tail -n 30 "$WORK/out/driftwm-build.log" >&2; exit 1; }
  # The binary must really have been built from the patched Smithay copy.
  grep -q 'Compiling smithay v[0-9.]* (/smithay)' "$WORK/out/driftwm-build.log" \
    || { echo "driftwm was not built against the patched Smithay copy" >&2; exit 1; }
  cp "$WORK/out/driftwm-target/release/driftwm" "$WORK/out/driftwm"
  umount "$BR/smithay" "$BR/src" "$BR/host/rustup" "$BR/host/cargo"
  printf '%s\n' "$PATCH_STAMP" > "$WORK/out/driftwm.stamp"
  say "driftwm built in $(( $(date +%s) - s )) s: $(stat -c %s "$WORK/out/driftwm") bytes (patch set $PATCH_STAMP)"
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
