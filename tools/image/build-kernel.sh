#!/bin/bash
# build-kernel.sh: build the machine's kernel (version and fragment come from the machine file).
# One kernel PER SLOT: the slot (hubos.slot=a|b and root=PARTLABEL=hubos-root-a|b) is in the built-in command
# line, so a firmware that drops the boot entry's load options still boots the right slot.
# Output: $WORK/out/kernel-a.efi and kernel-b.efi (a bzImage with an EFI stub and stage 0 built in),
#         $WORK/out/kernel.config (slot a's), kernel.version, kernel.seconds
. "$(dirname "$0")/common.sh"
load_machine
V=$KERNEL_VERSION
# Skip the build if nothing it depends on has changed (FORCE=1 builds anyway).
STAMP=$(cat "$REPO/$MACHINE" "$REPO/$KERNEL_FRAGMENT" "$REPO/image/stage0/init" "$REPO/image/stage0/stage0.list.in" | sha256sum | cut -d' ' -f1)
if [ -z "$FORCE" ] && [ -f "$WORK/out/kernel-a.efi" ] && [ -f "$WORK/out/kernel-b.efi" ] && [ "$(cat "$WORK/out/kernel.stamp" 2>/dev/null)" = "$STAMP" ]; then
  say "kernel is up to date ($(cat "$WORK/out/kernel.seconds") s when built)"; exit 0
fi
SRC=$WORK/linux-$V; KB=$WORK/kbuild-$V
if [ ! -d "$SRC" ]; then
  say "downloading linux-$V"
  curl -sS -o "$WORK/linux-$V.tar.xz" "https://cdn.kernel.org/pub/linux/kernel/v${V%%.*}.x/linux-$V.tar.xz"
  tar -C "$WORK" -xf "$WORK/linux-$V.tar.xz"
fi
# stage 0: the initramfs list built into the kernel
mkdir -p "$WORK/stage0"
sed -e "s#@BUSYBOX@#$T/usr/bin/busybox#" -e "s#@STAGE0@#$REPO/image/stage0/init#" "$REPO/image/stage0/stage0.list.in" > "$WORK/stage0/stage0.list"
FRAG=$WORK/stage0/kernel.frag
sed -e "s#@STAGE0_LIST@#$WORK/stage0/stage0.list#" -e "s#@SLOT@#a#g" "$REPO/$KERNEL_FRAGMENT" > "$FRAG"
HF=(HOSTCFLAGS="-I$T/usr/include" HOSTLDFLAGS="-L$T/usr/lib/x86_64-linux-gnu")
# Same output directory and the same build identity every time, so that two builds can be compared.
export KBUILD_BUILD_TIMESTAMP="@$SOURCE_DATE_EPOCH" KBUILD_BUILD_USER=hubos KBUILD_BUILD_HOST=hubos KBUILD_BUILD_VERSION=1
rm -rf "$KB"; mkdir -p "$KB"
say "kernel $V: tinyconfig + $KERNEL_FRAGMENT"
make -C "$SRC" O="$KB" tinyconfig "${HF[@]}" >/dev/null
cat "$FRAG" >> "$KB/.config"
make -C "$SRC" O="$KB" olddefconfig "${HF[@]}" >/dev/null
s=$(date +%s)
make -C "$SRC" O="$KB" -j"$(nproc)" bzImage "${HF[@]}" > "$WORK/out/kernel-build.log" 2>&1 || { tail -20 "$WORK/out/kernel-build.log" >&2; exit 1; }
cp "$KB/arch/x86/boot/bzImage" "$WORK/out/kernel-a.efi"
cp "$KB/.config" "$WORK/out/kernel.config"
# slot b: only the built-in command line differs; the second make rebuilds just what depends on it
grep -q 'hubos-root-a hubos.slot=a' "$KB/.config" || { echo "build-kernel.sh: no slot name in the built-in command line" >&2; exit 1; }
sed -i 's/hubos-root-a hubos.slot=a/hubos-root-b hubos.slot=b/' "$KB/.config"
make -C "$SRC" O="$KB" olddefconfig "${HF[@]}" >/dev/null
make -C "$SRC" O="$KB" -j"$(nproc)" bzImage "${HF[@]}" >> "$WORK/out/kernel-build.log" 2>&1 || { tail -20 "$WORK/out/kernel-build.log" >&2; exit 1; }
cp "$KB/arch/x86/boot/bzImage" "$WORK/out/kernel-b.efi"
echo $(( $(date +%s) - s )) > "$WORK/out/kernel.seconds"
echo "$V" > "$WORK/out/kernel.version"; echo "$STAMP" > "$WORK/out/kernel.stamp"
say "kernels built in $(cat "$WORK/out/kernel.seconds") s: $(stat -c %s "$WORK/out/kernel-a.efi") bytes, sha256 a $(sha256sum "$WORK/out/kernel-a.efi" | cut -c1-16) b $(sha256sum "$WORK/out/kernel-b.efi" | cut -c1-16)"
