#!/bin/bash
# usage: repro-run.sh BIN
ulimit -c 0
S=${HS_WORK}
L=$S/A/root/usr/lib/x86_64-linux-gnu
rm -rf /tmp/hsp; mkdir -p /tmp/hsp; chmod 700 /tmp/hsp
export XDG_RUNTIME_DIR=/tmp/hsp DISPLAY=:81 LD_LIBRARY_PATH=$L LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
export __EGL_VENDOR_LIBRARY_DIRS=$S/A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm
$1 --backend winit --config /dev/null > /tmp/hsp/dw.log 2>&1 &
P=$!
for i in $(seq 1 100); do [ -S /tmp/hsp/wayland-1 ] && break; sleep 0.1; done
echo "alive before: $(kill -0 $P 2>/dev/null && echo yes || echo no)"
python3 $S/hs/repro_shm.py /tmp/hsp/wayland-1
sleep 1
if kill -0 $P 2>/dev/null; then echo "alive after: yes (survived)"; kill $P; else wait $P; echo "alive after: NO, compositor exited with status $?"; fi
grep -m2 -E "panicked|TryFromInt" /tmp/hsp/dw.log | cut -c1-200
