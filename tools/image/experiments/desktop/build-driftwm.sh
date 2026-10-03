#!/bin/bash
# build-driftwm.sh: builds driftwm at the pinned commit (HUB-OS.md, docs/driftwm-findings.md) as a RELEASE build
# inside a throwaway build root made by mmdebstrap from the same Ubuntu snapshot as the image (so it links the
# image's own library versions). The host's Rust toolchain is bind-mounted into the build root (the snapshot's
# rustc is too old). Output: $WORK/out/driftwm. Needs root, network (crates.io, github.com). EXPERIMENT.
. "$(dirname "$0")/../../common.sh"
load_machine
COMMIT=352333a8fa1b22171492d4b71a54102045c9a19d
BR=$WORK/driftwm-buildroot
[ -x "$WORK/out/driftwm" ] && { say "driftwm already built"; exit 0; }
if [ ! -d "$BR/usr" ]; then
  say "build root (dev packages)"
  rm -rf "$BR"; mkdir -p "$BR"
  mmdebstrap --mode=root --variant=minbase --components=main,universe \
    --include=build-essential,pkg-config,git,ca-certificates,clang,libclang-dev,cmake,libwayland-dev,libxkbcommon-dev,libinput-dev,libudev-dev,libseat-dev,libgbm-dev,libegl-dev,libgles-dev,libdrm-dev,libdisplay-info-dev,libpixman-1-dev,libfontconfig-dev,libfreetype-dev \
    noble "$WORK/out/buildroot.tar" "http://snapshot.ubuntu.com/ubuntu/$SNAPSHOT" >/dev/null
  tar -C "$BR" --numeric-owner -xpf "$WORK/out/buildroot.tar"
fi
SRC=$WORK/driftwm-src
if [ ! -d "$SRC/.git" ]; then
  say "cloning driftwm"
  GIT_LFS_SKIP_SMUDGE=1 git clone -q --depth 50 https://github.com/malbiruk/driftwm "$SRC"
fi
git -C "$SRC" checkout -q "$COMMIT" || { git -C "$SRC" fetch -q --depth 500 origin "$COMMIT" && git -C "$SRC" checkout -q "$COMMIT"; }
say "driftwm at $(git -C "$SRC" rev-parse HEAD)"
CARGO_HOME_HOST=${CARGO_HOME:-$HOME/.cargo}; RUSTUP_HOST=${RUSTUP_HOME:-$HOME/.rustup}
mkdir -p "$BR/host/cargo" "$BR/host/rustup" "$BR/src" "$BR/out" "$BR/proc" "$BR/dev"
mount --bind "$CARGO_HOME_HOST" "$BR/host/cargo"; mount --bind "$RUSTUP_HOST" "$BR/host/rustup"
mount --bind "$SRC" "$BR/src"; mount --bind "$WORK/out" "$BR/out"; mount -t proc proc "$BR/proc"; mount --bind /dev "$BR/dev"
trap 'umount "$BR/dev" "$BR/proc" "$BR/out" "$BR/src" "$BR/host/rustup" "$BR/host/cargo" 2>/dev/null' EXIT
[ -f /root/.ccr/ca-bundle.crt ] && { mkdir -p "$BR/etc/ssl/certs"; cp /root/.ccr/ca-bundle.crt "$BR/etc/ssl/certs/ca-bundle-proxy.crt"; }
cp /etc/resolv.conf "$BR/etc/resolv.conf" 2>/dev/null || true
s=$(date +%s)
chroot "$BR" /usr/bin/env -i HOME=/root PATH=/host/cargo/bin:/usr/sbin:/usr/bin:/sbin:/bin CARGO_HOME=/host/cargo RUSTUP_HOME=/host/rustup \
  CARGO_NET_GIT_FETCH_WITH_CLI=true HTTPS_PROXY="${HTTPS_PROXY:-}" HTTP_PROXY="${HTTP_PROXY:-}" NO_PROXY="${NO_PROXY:-}" \
  SSL_CERT_FILE=/etc/ssl/certs/ca-bundle-proxy.crt GIT_SSL_CAINFO=/etc/ssl/certs/ca-bundle-proxy.crt CARGO_HTTP_CAINFO=/etc/ssl/certs/ca-bundle-proxy.crt \
  CARGO_TARGET_DIR=/out/driftwm-target SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" \
  /bin/sh -c 'cd /src && cargo build --release 2>&1' > "$WORK/out/driftwm-build.log" || { tail -n 30 "$WORK/out/driftwm-build.log" >&2; exit 1; }
cp "$WORK/out/driftwm-target/release/driftwm" "$WORK/out/driftwm"
say "driftwm built in $(( $(date +%s) - s )) s: $(stat -c %s "$WORK/out/driftwm") bytes"
