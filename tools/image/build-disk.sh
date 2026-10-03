#!/bin/bash
# build-disk.sh ROOT.sqsh KERNEL OUTDIR  ->  OUTDIR/disk.img (GPT: boot, slot A, slot B, config, data) and OUTDIR/vars.fd
# The disk holds root image ROOT in slot A and KERNEL as the slot A kernel and the fallback loader.
# No loop devices: each partition is its own file, put in place with dd.
. "$(dirname "$0")/common.sh"
load_machine
R=$1; K=$2; D=$3
[ -f "$R" ] && [ -f "$K" ] && [ -n "$D" ] || { echo "usage: build-disk.sh ROOT.sqsh KERNEL OUTDIR" >&2; exit 2; }
mkdir -p "$D"; rm -f "$D"/disk.img "$D"/*.part
# the config partition: node.conf from the machine file, inventory and viewers from image/config
C=$D/cfg.dir; rm -rf "$C"; mkdir -p "$C/hubos"
{ echo "NAME=hub-qemu"; echo "NET=dhcp"; echo "IFACE=eth0"; echo "SERVICES=\"$SERVICES\""; echo "CONFIRM_TIMEOUT=${CONFIRM_TIMEOUT:-40}"; } > "$C/hubos/node.conf"
cp "$REPO"/image/config/qemu-test/inventory.toml "$REPO"/image/config/qemu-test/viewers.toml "$C/hubos/"
truncate -s 64M "$D/esp.part"; mkfs.vfat -F 32 -n ESP "$D/esp.part" >/dev/null
export MTOOLS_SKIP_CHECK=1
mmd -i "$D/esp.part" ::/EFI ::/EFI/BOOT ::/EFI/hubos
mcopy -i "$D/esp.part" "$K" ::/EFI/BOOT/BOOTX64.EFI
mcopy -i "$D/esp.part" "$K" ::/EFI/hubos/kernel-a.efi
truncate -s 16M "$D/cfg.part"; mkfs.ext4 -q -L hubos-config -d "$C" "$D/cfg.part"
truncate -s 160M "$D/data.part"; mkfs.ext4 -q -L hubos-data "$D/data.part"
truncate -s 420M "$D/disk.img"
sfdisk -q "$D/disk.img" <<PT
label: gpt
start=2048, size=64MiB, type=U, name="hubos-esp"
size=64MiB, type=L, name="hubos-root-a"
size=64MiB, type=L, name="hubos-root-b"
size=16MiB, type=L, name="hubos-config"
size=160MiB, type=L, name="hubos-data"
PT
off() { sfdisk -J "$D/disk.img" | python3 -c "import json,sys; print(json.load(sys.stdin)['partitiontable']['partitions'][$1]['start'])"; }
dd if="$D/esp.part"  of="$D/disk.img" bs=512 seek="$(off 0)" conv=notrunc status=none
dd if="$R"           of="$D/disk.img" bs=512 seek="$(off 1)" conv=notrunc status=none
dd if="$D/cfg.part"  of="$D/disk.img" bs=512 seek="$(off 3)" conv=notrunc status=none
dd if="$D/data.part" of="$D/disk.img" bs=512 seek="$(off 4)" conv=notrunc status=none
rm -rf "$D"/*.part "$C"
cp "$T/usr/share/OVMF/OVMF_VARS_4M.fd" "$D/vars.fd"
say "disk $D/disk.img ready"
