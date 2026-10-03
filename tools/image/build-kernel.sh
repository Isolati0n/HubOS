#!/bin/bash
# build-kernel.sh: build the machine's kernel (version and fragment come from the machine file).
# Output: $WORK/out/kernel.efi (a bzImage with an EFI stub and stage 0 built in), $WORK/out/kernel.config,
#         $WORK/out/kernel.version, $WORK/out/kernel.seconds
. "$(dirname "$0")/common.sh"
load_machine
V=$KERNEL_VERSION
# Skip the build if nothing it depends on has changed (FORCE=1 builds anyway).
STAMP=$(cat "$REPO/$MACHINE" "$REPO/$KERNEL_FRAGMENT" "$REPO/image/stage0/init" "$REPO/image/stage0/stage0.list.in" | sha256sum | cut -d' ' -f1)
if [ -z "$FORCE" ] && [ -f "$WORK/out/kernel.efi" ] && [ "$(cat "$WORK/out/kernel.stamp" 2>/dev/null)" = "$STAMP" ]; then
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
sed "s#@STAGE0_LIST@#$WORK/stage0/stage0.list#" "$REPO/$KERNEL_FRAGMENT" > "$FRAG"
HF=(HOSTCFLAGS="-I$T/usr/include" HOSTLDFLAGS="-L$T/usr/lib/x86_64-linux-gnu")
# Same output directory and the same build identity every time, so that two builds can be compared.
export KBUILD_BUILD_TIMESTAMP="@$SOURCE_DATE_EPOCH" KBUILD_BUILD_USER=hubos KBUILD_BUILD_HOST=hubos
rm -rf "$KB"; mkdir -p "$KB"
say "kernel $V: tinyconfig + $KERNEL_FRAGMENT"
make -C "$SRC" O="$KB" tinyconfig "${HF[@]}" >/dev/null
cat "$FRAG" >> "$KB/.config"
make -C "$SRC" O="$KB" olddefconfig "${HF[@]}" >/dev/null
s=$(date +%s)
make -C "$SRC" O="$KB" -j"$(nproc)" bzImage "${HF[@]}" > "$WORK/out/kernel-build.log" 2>&1 || { tail -20 "$WORK/out/kernel-build.log" >&2; exit 1; }
echo $(( $(date +%s) - s )) > "$WORK/out/kernel.seconds"
cp "$KB/arch/x86/boot/bzImage" "$WORK/out/kernel.efi"
cp "$KB/.config" "$WORK/out/kernel.config"
echo "$V" > "$WORK/out/kernel.version"; echo "$STAMP" > "$WORK/out/kernel.stamp"
say "kernel built in $(cat "$WORK/out/kernel.seconds") s: $(stat -c %s "$WORK/out/kernel.efi") bytes, sha256 $(sha256sum "$WORK/out/kernel.efi" | cut -c1-16)"
