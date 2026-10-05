#!/bin/bash
# config-check.sh: what does the repo's kernel configuration really switch on for the crash-evidence features?
# Builds ONLY the .config (no compile): `make tinyconfig`, append image/kernel/<name>.frag, `make olddefconfig`
# (the same three steps tools/image/build-kernel.sh does), then prints the lines about pstore, EFI variables, watchdog.
# Usage: WORK=dir tools/image/experiments/black-box/config-check.sh [qemu-test|hub]   (WORK must hold linux-6.12 and tools/root from fetch-tools.sh)
set -e
. "$(dirname "$0")/../../common.sh"
FR=${1:-qemu-test}
V=6.12; KB=$WORK/kbuild-cfg-$FR; rm -rf "$KB"; mkdir -p "$KB"
make -C "$WORK/linux-$V" O="$KB" tinyconfig HOSTCFLAGS="-I$T/usr/include" HOSTLDFLAGS="-L$T/usr/lib/x86_64-linux-gnu" >/dev/null
sed -e 's#@STAGE0_LIST@#/dev/null#' -e 's#@SLOT@#a#g' -e 's#@WATCHDOG@#60#g' "$REPO/image/kernel/$FR.frag" >> "$KB/.config"
make -C "$WORK/linux-$V" O="$KB" olddefconfig HOSTCFLAGS="-I$T/usr/include" HOSTLDFLAGS="-L$T/usr/lib/x86_64-linux-gnu" >/dev/null
echo "== $FR: pstore / EFI / watchdog / panic lines of the finished .config =="
grep -E "PSTORE|EFI_VARS|CONFIG_EFI=|EFIVAR|WATCHDOG|I6300|PANIC|RAMOOPS|MEMMAP|CMDLINE|CONFIG_CRASH|KEXEC|LOG_BUF|MAGIC_SYSRQ" "$KB/.config" | sort
echo "== PSTORE lines counted: $(grep -c '^CONFIG_PSTORE' "$KB/.config") =="
rm -rf "$KB"
