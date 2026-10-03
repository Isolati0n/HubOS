#!/bin/bash
# build-bundle.sh VERSION FLAVOR OUTDIR PUBKEY SECKEY
#   OUTDIR gets manifest, manifest.sig, kernel-a.efi, kernel-b.efi (one kernel per slot), rootfs.sqsh (which carries
#   the separate recovery kernel in /usr/lib/hubos/recovery). The manifest names the kernel version, the module list, the recovery
#   version and the hash of every kernel, and is signed with SECKEY (the runner's throwaway key).
#   NO_KERNEL_VERSION=1 leaves the kernel-version line out (a bundle the update tool must refuse).
#   NO_RECOVERY=1 leaves the recovery kernel (in the root and in the manifest) out.
#   RECOVERY_VERSION_OVERRIDE=N writes another recovery-version; RECOVERY_KERNEL_FILE=PATH uses another file.
. "$(dirname "$0")/common.sh"
load_machine
V=$1; F=$2; D=$3; PUB=$4; SEC=$5
[ -n "$SEC" ] || { echo "usage: build-bundle.sh VERSION FLAVOR OUTDIR PUBKEY SECKEY" >&2; exit 2; }
mkdir -p "$D"
# a garbage root has no recovery kernel inside it, so its manifest lists none
[ "$F" = garbage ] && export NO_RECOVERY=1
"$(dirname "$0")/build-root-image.sh" "$V" "$F" "$D/rootfs.sqsh" "$PUB"
cp "$WORK/out/kernel-a.efi" "$D/kernel-a.efi"; cp "$WORK/out/kernel-b.efi" "$D/kernel-b.efi"
# The recovery kernel is inside the root (build-root-image.sh); the manifest lists its hash and version but the
# bundle carries no separate file: the update tool checks the file inside the written root against the manifest.
RK=${RECOVERY_KERNEL_FILE:-$WORK/out/kernel-recovery.efi}
{
  echo "hubos-bundle 1"; echo "version $V"; echo "flavor $F"; echo "arch x86_64"
  [ -n "$NO_KERNEL_VERSION" ] || echo "kernel-version $KERNEL_VERSION"
  echo "modules $(echo "$MODULES" | tr ' ' ',')"
  [ -n "$NO_RECOVERY" ] || echo "recovery-version ${RECOVERY_VERSION_OVERRIDE:-${RECOVERY_VERSION:-1}}"
  for f in kernel-a.efi kernel-b.efi rootfs.sqsh; do echo "$f sha256 $(sha256sum "$D/$f" | cut -d' ' -f1) size $(stat -c %s "$D/$f")"; done
  [ -n "$NO_RECOVERY" ] || echo "kernel-recovery.efi sha256 $(sha256sum "$RK" | cut -d' ' -f1) size $(stat -c %s "$RK")"
} > "$D/manifest"
signify-openbsd -S -s "$SEC" -m "$D/manifest" -x "$D/manifest.sig"
