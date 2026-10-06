#!/bin/bash
# usage: termtest.sh BIN CLIENTS(0/1)
ulimit -c 0
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
BIN=$1
RTD=/tmp/bc4term; rm -rf $RTD $W/term; mkdir -p $RTD $W/term; chmod 700 $RTD
export XDG_RUNTIME_DIR=$RTD DISPLAY=:93 LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
export __EGL_VENDOR_LIBRARY_DIRS=$A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm
export XDG_STATE_HOME=$W/term/state XDG_DATA_HOME=$W/term/data PATH=$A/root/usr/bin:$PATH
unset WAYLAND_DISPLAY
echo '' > $W/term/c.toml
cd $W/term
$BIN --backend winit --config $W/term/c.toml > $W/term/dw.log 2>&1 &
DPID=$!
for i in $(seq 1 150); do [ -S $RTD/wayland-1 ] && break; sleep 0.1; done
if [ "$2" = "1" ]; then WAYLAND_DISPLAY=wayland-1 foot sleep 100 & sleep 2; fi
echo "sending SIGTERM to $DPID"; kill -TERM $DPID
for i in $(seq 1 100); do kill -0 $DPID 2>/dev/null || { echo "exited after $i x 0.1s"; break; }; sleep 0.1; done
kill -0 $DPID 2>/dev/null && { echo "STILL RUNNING after 10s; state:"; grep -E "State|SigBlk|SigCgt" /proc/$DPID/status; cat /proc/$DPID/wchan; echo; kill -9 $DPID; }
tail -3 $W/term/dw.log | cut -c1-200
