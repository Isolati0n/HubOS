#!/bin/bash
# build-lab.sh: build the black box recorder LAB kernel: the repo's tinyconfig + image/kernel/qemu-test.frag (same steps as
# tools/image/build-kernel.sh) + lab.frag (pstore) and a tiny initramfs (lab-init). The built-in command line is cut down to what
# the experiments need (no panic=, no root=), so the harness chooses panic= and the pstore options on the QEMU command line.
# Usage: WORK=dir build-lab.sh      (WORK holds linux-6.12 and tools/root from tools/image/fetch-tools.sh). Output: $WORK/lab/bzImage
set -e
. "$(dirname "$0")/../../common.sh"
HERE=$(cd "$(dirname "$0")" && pwd)
V=6.12; L=$WORK/lab; KB=$L/kbuild; mkdir -p "$L"
cat > "$L/initramfs.list" <<EOL
dir /dev 755 0 0
dir /proc 755 0 0
dir /sys 755 0 0
dir /bin 755 0 0
dir /cfg 755 0 0
nod /dev/console 600 0 0 c 5 1
file /bin/busybox $T/usr/bin/busybox 755 0 0
file /init $HERE/lab-init 755 0 0
file /bin/blackbox-save.sh $HERE/blackbox-save.sh 755 0 0
file /bin/blackbox-logs.sh $HERE/blackbox-logs.sh 755 0 0
EOL
HF=(HOSTCFLAGS="-I$T/usr/include" HOSTLDFLAGS="-L$T/usr/lib/x86_64-linux-gnu")
if [ -z "$INIT_ONLY" ] || [ ! -f "$KB/.config" ]; then   # INIT_ONLY=1: only the initramfs (lab-init) changed, keep the configured tree
rm -rf "$KB"; mkdir -p "$KB"
make -C "$WORK/linux-$V" O="$KB" tinyconfig "${HF[@]}" >/dev/null
sed -e "s#@STAGE0_LIST@#$L/initramfs.list#" -e 's#@SLOT@#a#g' -e 's#@WATCHDOG@#60#g' "$REPO/image/kernel/qemu-test.frag" \
  | sed -e 's#^CONFIG_CMDLINE=.*#CONFIG_CMDLINE="console=ttyS0 loglevel=7"#' >> "$KB/.config"
cat "$HERE/lab.frag" >> "$KB/.config"
make -C "$WORK/linux-$V" O="$KB" olddefconfig "${HF[@]}" >/dev/null 2>&1
fi
grep -E '^CONFIG_(PSTORE|EFI_VARS_PSTORE)' "$KB/.config"
make -C "$WORK/linux-$V" O="$KB" -j"$(nproc)" bzImage "${HF[@]}" > "$L/build.log" 2>&1 || { tail -20 "$L/build.log"; exit 1; }
cp "$KB/arch/x86/boot/bzImage" "$L/bzImage"; cp "$KB/.config" "$L/lab.config"
ls -l "$L/bzImage"
