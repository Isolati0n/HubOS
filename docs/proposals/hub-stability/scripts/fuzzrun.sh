#!/bin/bash
# usage: fuzzrun.sh BIN SECONDS SEED OUTDIR
ulimit -c 0
S=${HS_WORK}
BIN=$1; SECS=$2; SEED=$3; OUT=$4
rm -rf $OUT /tmp/hsf; mkdir -p $OUT /tmp/hsf; chmod 700 /tmp/hsf
L=$S/A/root/usr/lib/x86_64-linux-gnu
export XDG_RUNTIME_DIR=/tmp/hsf DISPLAY=:81 LD_LIBRARY_PATH=$L LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
export __EGL_VENDOR_LIBRARY_DIRS=$S/A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm RUST_BACKTRACE=1
export XDG_STATE_HOME=$OUT/state XDG_DATA_HOME=$OUT/data
echo '' > $OUT/c.toml
$BIN --backend winit --config $OUT/c.toml > $OUT/dw.log 2>&1 &
DPID=$!
for i in $(seq 1 100); do [ -S /tmp/hsf/wayland-1 ] && break; sleep 0.1; done
python3 $S/hs/fuzz.py /tmp/hsf/wayland-1 $DPID $SECS $OUT $SEED 2>&1 | tail -25
if kill -0 $DPID 2>/dev/null; then python3 $S/hs/healthcheck.py /tmp/hsf; echo "COMPOSITOR ALIVE after fuzz; VmHWM/RSS:"; grep -E "VmHWM|VmRSS" /proc/$DPID/status; ls /proc/$DPID/fd | wc -l; kill $DPID; else echo "COMPOSITOR DIED"; fi
grep -E "panicked|RUST_BACKTRACE|thread '" $OUT/dw.log | head -10
