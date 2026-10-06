#!/bin/bash
# usage: reprorun.sh BIN LABEL CMD...    (the word SOCK in CMD is replaced by the Wayland socket path)
# starts the compositor, runs CMD, then reports: alive? answers wl_display.sync? exit status; backtrace of the main thread if hung.
ulimit -c 0
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
BIN=$1; LABEL=$2; shift 2
RTD=/tmp/bc4r-$LABEL; OUT=$W/rr/$LABEL
rm -rf $RTD $OUT; mkdir -p $RTD $OUT; chmod 700 $RTD
export XDG_RUNTIME_DIR=$RTD DISPLAY=${FZDISPLAY:-:93} LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
export __EGL_VENDOR_LIBRARY_DIRS=$A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm RUST_BACKTRACE=1
export XDG_STATE_HOME=$OUT/state XDG_DATA_HOME=$OUT/data PATH=$A/root/usr/bin:$PATH
unset WAYLAND_DISPLAY
echo '' > $OUT/c.toml
cd $OUT
[ -n "$RLIM_V" ] && ulimit -v $RLIM_V
$BIN --backend winit --config $OUT/c.toml > $OUT/dw.log 2>&1 &
DPID=$!
for i in $(seq 1 150); do [ -S $RTD/wayland-1 ] && break; sleep 0.1; done
sleep 0.5
args=()
for a in "$@"; do a="${a//SOCK/$RTD/wayland-1}"; args+=("${a//DPID/$DPID}"); done
timeout 60 "${args[@]}"
sleep 1.5
if kill -0 $DPID 2>/dev/null; then
  if timeout 8 python3 -c "
import socket,struct
s=socket.socket(socket.AF_UNIX); s.settimeout(5); s.connect('$RTD/wayland-1'); s.sendall(struct.pack('<III',1,(12<<16)|0,2)); d=s.recv(100); assert len(d)>=8
"; then echo "RESULT[$LABEL]: compositor ALIVE and answering"; else
    echo "RESULT[$LABEL]: compositor ALIVE BUT HUNG (state: $(grep State /proc/$DPID/status | tr '\t' ' '), cpu ticks $(awk '{print $14+$15}' /proc/$DPID/stat))"
    timeout 100 gdb -p $DPID -batch -ex "thread 1" -ex "bt 14" 2>&1 | grep -E "^#" | sed -E 's#/tmp/claude[^ ]*scratchpad/bc4/##g' | cut -c1-200 | head -16
  fi
  kill -9 $DPID
else
  wait $DPID; echo "RESULT[$LABEL]: compositor DEAD, exit status $?"; grep -E "panicked" -A3 $OUT/dw.log | head -6 | cut -c1-250
fi
rm -rf $RTD
