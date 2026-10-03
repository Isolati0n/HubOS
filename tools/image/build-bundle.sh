#!/bin/bash
# build-bundle.sh VERSION FLAVOR OUTDIR PUBKEY SECKEY
#   OUTDIR gets manifest, manifest.sig, kernel.efi, rootfs.sqsh. The manifest names the kernel version and
#   the module list and is signed with SECKEY (the runner's throwaway key). NO_KERNEL_VERSION=1 leaves the
#   kernel-version line out (a bundle the update tool must refuse).
. "$(dirname "$0")/common.sh"
load_machine
V=$1; F=$2; D=$3; PUB=$4; SEC=$5
[ -n "$SEC" ] || { echo "usage: build-bundle.sh VERSION FLAVOR OUTDIR PUBKEY SECKEY" >&2; exit 2; }
mkdir -p "$D"
"$(dirname "$0")/build-root-image.sh" "$V" "$F" "$D/rootfs.sqsh" "$PUB"
cp "$WORK/out/kernel.efi" "$D/kernel.efi"
{
  echo "hubos-bundle 1"; echo "version $V"; echo "flavor $F"; echo "arch x86_64"
  [ -n "$NO_KERNEL_VERSION" ] || echo "kernel-version $KERNEL_VERSION"
  echo "modules $(echo "$MODULES" | tr ' ' ',')"
  for f in kernel.efi rootfs.sqsh; do echo "$f sha256 $(sha256sum "$D/$f" | cut -d' ' -f1) size $(stat -c %s "$D/$f")"; done
} > "$D/manifest"
signify-openbsd -S -s "$SEC" -m "$D/manifest" -x "$D/manifest.sig"
