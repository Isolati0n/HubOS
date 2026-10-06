#!/bin/bash
# usage: fuzzone.sh BIN SECONDS SEED OUTDIR [FUZZ_SKIP] [extra env assignments via FZENV]
ulimit -c 0
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
BIN=$1; SECS=$2; SEED=$3; OUT=$4
RTD=/tmp/bc4f$SEED
rm -rf $OUT $RTD; mkdir -p $OUT $RTD; chmod 700 $RTD
export XDG_RUNTIME_DIR=$RTD DISPLAY=${FZDISPLAY:-:93} LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
export __EGL_VENDOR_LIBRARY_DIRS=$A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm RUST_BACKTRACE=1
export XDG_STATE_HOME=$OUT/state XDG_DATA_HOME=$OUT/data PATH=$A/root/usr/bin:$PATH
unset WAYLAND_DISPLAY
echo '' > $OUT/c.toml
cd $OUT
if [ -n "$GDB" ]; then
  nice -n 10 gdb -batch -ex 'handle SIGPIPE nostop noprint' -ex run -ex 'bt 30' -ex 'thread apply all bt 6' --args $BIN --backend winit --config $OUT/c.toml > $OUT/dw.log 2>&1 &
else
  nice -n 10 $BIN --backend winit --config $OUT/c.toml > $OUT/dw.log 2>&1 &
fi
DPID=$!
for i in $(seq 1 150); do [ -S $RTD/wayland-1 ] && break; sleep 0.1; done
FUZZ_SKIP="${FUZZ_SKIP:-}" python3 $W/fuzz2.py $RTD/wayland-1 $DPID $SECS $OUT $SEED $W/proto/all 2>&1 | tail -25
if kill -0 $DPID 2>/dev/null; then
  echo "COMPOSITOR ALIVE after fuzz; VmHWM/RSS:"; grep -E "VmHWM|VmRSS" /proc/$DPID/status; echo "fds: $(ls /proc/$DPID/fd | wc -l) threads: $(ls /proc/$DPID/task | wc -l)"
  python3 - <<PYEOF
import socket,glob,time,json
t=time.time()
try:
    s=socket.socket(socket.AF_UNIX); s.settimeout(8); s.connect(glob.glob('$RTD/driftwm/ipc-*.sock')[0]); s.sendall(b'\"State\"\n'); b=b''
    while not b.endswith(b'\n'): b+=s.recv(1<<20)
    j=json.loads(b); print('POST-FUZZ IPC State ok in %.3fs, windows=%d' % (time.time()-t, len(j['Ok']['State']['windows'])))
except Exception as e: print('POST-FUZZ IPC FAILED', type(e).__name__, e)
PYEOF
  if ! timeout 6 python3 -c "
import socket,glob
s=socket.socket(socket.AF_UNIX); s.settimeout(4); s.connect(glob.glob('$RTD/driftwm/ipc-*.sock')[0]); s.sendall(b'\"Zoom\"\n'); s.recv(100)" 2>/dev/null; then echo "HUNG: taking backtrace with gdb"; timeout 120 gdb -p $DPID -batch -ex "thread apply all bt 30" > $OUT/hang-bt.txt 2>&1; grep -E "^#" $OUT/hang-bt.txt | head -25; fi
  kill -TERM $DPID
  for i in $(seq 1 100); do kill -0 $DPID 2>/dev/null || { echo "SIGTERM: exited after $i x 0.1s"; break; }; sleep 0.1; done
  if kill -0 $DPID 2>/dev/null; then echo "SIGTERM IGNORED for 10 s: $(grep -E 'State' /proc/$DPID/status) wchan=$(cat /proc/$DPID/wchan) cpu_ticks=$(awk '{print $14+$15}' /proc/$DPID/stat)"; cp /proc/$DPID/status $OUT/stuck-status.txt; ls /proc/$DPID/task | head -20 > $OUT/stuck-tasks.txt; kill -9 $DPID; fi
else
  echo "COMPOSITOR DIED"
fi
grep -E "panicked|SIGSEGV|SIGABRT|AddressSanitizer|ThreadSanitizer|ERROR: " $OUT/dw.log | head -10
grep -A12 "panicked" $OUT/dw.log | head -30
rm -rf $RTD
