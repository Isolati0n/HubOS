#!/bin/bash
# common.sh: shared by the image scripts. Source it:  . "$(dirname "$0")/common.sh"
#   WORK     a temporary directory (required); everything the scripts make is in it
#   MACHINE  a machine build file, relative to the repo (default image/machines/qemu-test.build)
# Nothing is installed on the machine: tools are unpacked into $WORK/tools.
set -e
REPO=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
: "${WORK:?set WORK to a temporary directory}"
WORK=$(cd "$WORK" && pwd)
MACHINE=${MACHINE:-image/machines/qemu-test.build}
T=$WORK/tools/root
export LD_LIBRARY_PATH=$T/usr/lib/x86_64-linux-gnu:$T/lib/x86_64-linux-gnu:$T/usr/lib
export QEMU_MODULE_DIR=$T/usr/lib/x86_64-linux-gnu/qemu
export PATH=$T/usr/bin:$T/usr/sbin:$T/sbin:$T/bin:$PATH
export PERL5LIB=$T/usr/share/perl5:$T/usr/lib/x86_64-linux-gnu/perl5/5.38:$T/usr/lib/x86_64-linux-gnu/perl5
# Reproducibility: every file time and timestamp in the images comes from this (mksquashfs, mmdebstrap and
# the kernel build all read it; mksquashfs refuses time options when it is set).
export SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH:-1790000000}
mkdir -p "$WORK/out"
load_machine() {
  # shellcheck disable=SC1090
  . "$REPO/$MACHINE"
  : "${KERNEL_VERSION:?}" "${KERNEL_FRAGMENT:?}" "${PACKAGES:?}" "${SNAPSHOT:?}"
  MODULES=${MODULES:-}
}
say() { printf '%s %s\n' "$(date +%H:%M:%S)" "$*" >&2; }
