#!/bin/bash
# fetch-tools.sh: download the build and test tools from the Ubuntu 24.04 archive and
# unpack them into $WORK/tools/root with dpkg -x. Nothing is installed.
. "$(dirname "$0")/common.sh"
A=$WORK/tools
[ -x "$T/usr/bin/qemu-system-x86_64" ] && [ -f "$A/.done" ] && { say "tools already unpacked"; exit 0; }
mkdir -p "$A/apt/lists/partial" "$A/apt/cache/archives/partial" "$A/debs" "$T"
O="-o Dir::State::lists=$A/apt/lists -o Dir::Cache=$A/apt/cache -o APT::Sandbox::User=root"
say "apt-get update (own index in $A/apt)"
apt-get $O update >/dev/null 2>&1
say "listing packages"
# shellcheck disable=SC2086
apt-get $O install --print-uris -y --no-install-recommends \
  qemu-system-x86 seabios qemu-system-data ovmf ipxe-qemu efibootmgr efivar busybox-static squashfs-tools \
  s6 execline dosfstools mtools fdisk signify-openbsd flex cpio libelf-dev libelf1t64 \
  mmdebstrap libssl-dev bc libzstd-dev xz-utils libbsd0 \
  | grep -oE "^'[^']+'" | tr -d "'" > "$A/uris.txt"
say "downloading $(wc -l < "$A/uris.txt") packages"
( cd "$A/debs" && while read -r u; do [ -f "$(basename "$u")" ] || curl -sS -O "$u"; done < "$A/uris.txt" )
for d in "$A"/debs/*.deb; do dpkg -x "$d" "$T"; done
# libbsd0 is installed on most machines, so it is not in the list above
( cd "$A/debs" && apt-get $O download libbsd0 >/dev/null 2>&1 && dpkg -x libbsd0_*.deb "$T" ) || true
# the kernel build wants libelf.so
ln -sf /usr/lib/x86_64-linux-gnu/libelf.so.1 "$T/usr/lib/x86_64-linux-gnu/libelf.so" 2>/dev/null || true
touch "$A/.done"
say "tools ready: $(qemu-system-x86_64 --version | head -1)"
