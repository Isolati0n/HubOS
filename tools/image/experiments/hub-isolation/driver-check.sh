#!/bin/sh
# THROWAWAY experiment for docs/proposals/distro-workshop.md section 7.2 (the "driver check" step of the preflight).
# usage: driver-check.sh KERNEL_CONFIG PROFILE
#   KERNEL_CONFIG  a finished kernel .config (tools/image/build-kernel.sh writes $WORK/out/kernel.config)
#   PROFILE        lines "device: SYMBOL SYMBOL ..." (kernel option names without the CONFIG_ prefix); '#' starts a comment.
#                  A leading '!' on the device name means boot-critical: stage 0 loads no modules, so only 'y' is enough.
# For each symbol prints y (built in), m (module) or MISSING. Exit status 1 if any symbol is MISSING or a boot-critical one is a module.
cfg=$1; prof=$2
[ -f "$cfg" ] && [ -f "$prof" ] || { echo "usage: $0 KERNEL_CONFIG PROFILE" >&2; exit 2; }
res=$(sed -e 's/#.*//' -e '/^[[:space:]]*$/d' "$prof" | while IFS=: read -r dev syms; do
  need_y=0
  case $dev in '!'*) need_y=1; dev=${dev#!};; esac
  for s in $syms; do
    v=$(sed -n "s/^CONFIG_$s=\(.*\)/\1/p" "$cfg" | head -n 1)
    case $v in
      y) r=y ;;
      m) if [ $need_y = 1 ]; then r="m-BUT-MUST-BE-BUILT-IN"; else r=m; fi ;;
      *) r=MISSING ;;
    esac
    printf '%-26s %-22s %s\n' "$dev" "CONFIG_$s" "$r"
  done
done)
echo "$res"
case $res in *MISSING*|*MUST-BE-BUILT-IN*) exit 1;; esac
exit 0
