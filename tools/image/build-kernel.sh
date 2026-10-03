#!/bin/bash
# build-kernel.sh: build the machine's kernel (version and fragment come from the machine file).
# THREE kernels: one per slot and a separate recovery kernel (see below).
# One kernel PER SLOT: the slot (hubos.slot=a|b and root=PARTLABEL=hubos-root-a|b) is in the built-in command
# line, so a firmware that drops the boot entry's load options still boots the right slot.
# The RECOVERY kernel (kernel-recovery.efi) has a different initramfs (image/stage0/recovery-init and the tools
# listed by tools/image/recovery-list.py, copied from the base root) and no root= in its command line: it needs
# neither slot's root. It also carries the owner's PUBLIC update key ($UPDATE_PUB, a signify public key file), so
# that `hubos-ctl update` works in the recovery shell. The key is not in the repo: the caller passes it.
# Output: $WORK/out/kernel-a.efi, kernel-b.efi and kernel-recovery.efi (a bzImage with an EFI stub and stage 0 built in),
#         $WORK/out/kernel.config (slot a's), kernel.version, kernel.seconds
. "$(dirname "$0")/common.sh"
load_machine
V=$KERNEL_VERSION
# Per-machine settings: the first watchdog heartbeat is the machine's watchdog timeout; the pair is checked.
CT=${CONFIRM_TIMEOUT:-120}; WT=${WATCHDOG_TIMEOUT:-180}
"$REPO/image/rootfs/usr/lib/hubos/check-timers.sh" "$CT" "$WT" >&2 || { echo "build-kernel.sh: the machine's timeouts are refused" >&2; exit 1; }
# The recovery initramfs is made from the base root, so the base root must exist (build-base.sh is idempotent).
[ -d "$WORK/base" ] || "$(dirname "$0")/build-base.sh"
[ -n "$UPDATE_PUB" ] && [ -f "$UPDATE_PUB" ] || say "WARNING: UPDATE_PUB is not set: the recovery kernel is built WITHOUT an update key (hubos-ctl update will not work in recovery)"
# Two stamps. The slot kernels depend on the machine, the fragment and stage 0; the recovery kernel on its own files,
# the base root and the update key. Skip what has not changed (FORCE=1 builds everything).
SSTAMP=$(cat "$REPO/$MACHINE" "$REPO/$KERNEL_FRAGMENT" "$REPO/image/stage0/init" "$REPO/image/stage0/stage0.list.in" "$REPO/image/rootfs/usr/lib/hubos/check-timers.sh" | sha256sum | cut -d' ' -f1)
RSTAMP=$(cat "$REPO/image/stage0/recovery-init" "$REPO/image/stage0/recovery.rc" "$REPO/tools/image/recovery-list.py" "$REPO/image/rootfs/usr/sbin/hubos-ctl" "$REPO/image/rootfs/usr/lib/hubos/udhcpc.script" "$WORK/out/base.stamp" ${UPDATE_PUB:+"$UPDATE_PUB"} | sha256sum | cut -d' ' -f1)
RSTAMP=$(echo "$SSTAMP $RSTAMP ${RECOVERY_VERSION:-1} ${RECOVERY_CMDLINE:-}" | sha256sum | cut -d' ' -f1)
SRC=$WORK/linux-$V; KB=$WORK/kbuild-$V
SLOTS_OK=; REC_OK=
[ -z "$FORCE" ] && [ -f "$WORK/out/kernel-a.efi" ] && [ -f "$WORK/out/kernel-b.efi" ] && [ -f "$KB/.config" ] && [ "$(cat "$WORK/out/kernel.sstamp" 2>/dev/null)" = "$SSTAMP" ] && SLOTS_OK=1
[ -n "$SLOTS_OK" ] && [ -f "$WORK/out/kernel-recovery.efi" ] && [ "$(cat "$WORK/out/kernel.rstamp" 2>/dev/null)" = "$RSTAMP" ] && REC_OK=1
if [ -n "$SLOTS_OK" ] && [ -n "$REC_OK" ]; then
  say "kernels are up to date ($(cat "$WORK/out/kernel.seconds") s when built)"; exit 0
fi
if [ ! -d "$SRC" ]; then
  say "downloading linux-$V"
  curl -sS -o "$WORK/linux-$V.tar.xz" "https://cdn.kernel.org/pub/linux/kernel/v${V%%.*}.x/linux-$V.tar.xz"
  tar -C "$WORK" -xf "$WORK/linux-$V.tar.xz"
fi
HF=(HOSTCFLAGS="-I$T/usr/include" HOSTLDFLAGS="-L$T/usr/lib/x86_64-linux-gnu")
# Same output directory and the same build identity every time, so that two builds can be compared.
export KBUILD_BUILD_TIMESTAMP="@$SOURCE_DATE_EPOCH" KBUILD_BUILD_USER=hubos KBUILD_BUILD_HOST=hubos KBUILD_BUILD_VERSION=1
s=$(date +%s)
if [ -z "$SLOTS_OK" ]; then
  # stage 0: the initramfs list built into the kernel
  mkdir -p "$WORK/stage0"
  sed -e "s#@BUSYBOX@#$T/usr/bin/busybox#" -e "s#@STAGE0@#$REPO/image/stage0/init#" -e "s#@CHECKTIMERS@#$REPO/image/rootfs/usr/lib/hubos/check-timers.sh#" "$REPO/image/stage0/stage0.list.in" > "$WORK/stage0/stage0.list"
  FRAG=$WORK/stage0/kernel.frag
  sed -e "s#@STAGE0_LIST@#$WORK/stage0/stage0.list#" -e "s#@SLOT@#a#g" -e "s#@WATCHDOG@#$WT#g" "$REPO/$KERNEL_FRAGMENT" > "$FRAG"
  rm -rf "$KB"; mkdir -p "$KB"
  say "kernel $V: tinyconfig + $KERNEL_FRAGMENT"
  make -C "$SRC" O="$KB" tinyconfig "${HF[@]}" >/dev/null
  cat "$FRAG" >> "$KB/.config"
  make -C "$SRC" O="$KB" olddefconfig "${HF[@]}" >/dev/null
  make -C "$SRC" O="$KB" -j"$(nproc)" bzImage "${HF[@]}" > "$WORK/out/kernel-build.log" 2>&1 || { tail -20 "$WORK/out/kernel-build.log" >&2; exit 1; }
  cp "$KB/arch/x86/boot/bzImage" "$WORK/out/kernel-a.efi"
  cp "$KB/.config" "$WORK/out/kernel.config"
  # slot b: only the built-in command line differs; the second make rebuilds just what depends on it
  grep -q 'hubos-root-a hubos.slot=a' "$KB/.config" || { echo "build-kernel.sh: no slot name in the built-in command line" >&2; exit 1; }
  sed -i 's/hubos-root-a hubos.slot=a/hubos-root-b hubos.slot=b/' "$KB/.config"
  make -C "$SRC" O="$KB" olddefconfig "${HF[@]}" >/dev/null
  make -C "$SRC" O="$KB" -j"$(nproc)" bzImage "${HF[@]}" >> "$WORK/out/kernel-build.log" 2>&1 || { tail -20 "$WORK/out/kernel-build.log" >&2; exit 1; }
  cp "$KB/arch/x86/boot/bzImage" "$WORK/out/kernel-b.efi"
  echo "$SSTAMP" > "$WORK/out/kernel.sstamp"
fi
# the recovery kernel: another initramfs and a command line without root=/hubos.slot=
mkdir -p "$WORK/stage0"
printf 'version=recovery-%s\nflavor=recovery\nkernel-version=%s\n' "${RECOVERY_VERSION:-1}" "$V" > "$WORK/stage0/recovery-release"
( cd "$REPO" && python3 tools/image/recovery-list.py "$WORK/base" "$T" "$T/usr/bin/busybox" "$WORK/stage0/recovery-release" ${UPDATE_PUB:+"$UPDATE_PUB"} ) > "$WORK/stage0/recovery.list"
sed -i "s#^CONFIG_INITRAMFS_SOURCE=.*#CONFIG_INITRAMFS_SOURCE=\"$WORK/stage0/recovery.list\"#; s#^CONFIG_CMDLINE=.*#CONFIG_CMDLINE=\"${RECOVERY_CMDLINE:-console=ttyS0 ro loglevel=4 panic=5}\"#" "$KB/.config"
make -C "$SRC" O="$KB" olddefconfig "${HF[@]}" >/dev/null
make -C "$SRC" O="$KB" -j"$(nproc)" bzImage "${HF[@]}" >> "$WORK/out/kernel-build.log" 2>&1 || { tail -20 "$WORK/out/kernel-build.log" >&2; exit 1; }
cp "$KB/arch/x86/boot/bzImage" "$WORK/out/kernel-recovery.efi"
echo $(( $(date +%s) - s )) > "$WORK/out/kernel.seconds"
echo "$V" > "$WORK/out/kernel.version"; echo "$RSTAMP" > "$WORK/out/kernel.rstamp"
say "kernels (a, b, recovery) built in $(cat "$WORK/out/kernel.seconds") s ($([ -n "$SLOTS_OK" ] && echo 'recovery only' || echo 'all three')): $(stat -c %s "$WORK/out/kernel-a.efi") bytes, sha256 a $(sha256sum "$WORK/out/kernel-a.efi" | cut -c1-16) b $(sha256sum "$WORK/out/kernel-b.efi" | cut -c1-16) recovery $(stat -c %s "$WORK/out/kernel-recovery.efi") bytes $(sha256sum "$WORK/out/kernel-recovery.efi" | cut -c1-16)"
