#!/bin/bash
# a session.json whose single entry has a huge size: does the compositor survive the start?   usage: sessbig.sh BIN
ulimit -c 0
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
BIN=$1
for SZ in 4000 20000 32768 40000 2000000000; do
  RTD=/tmp/bc4sb; rm -rf $RTD $W/sb; mkdir -p $RTD $W/sb/data/applications; chmod 700 $RTD
  printf "[Desktop Entry]\nType=Application\nName=s01\nExec=foot --app-id=hubos-s01 sleep 100\nStartupWMClass=hubos-s01\n" > $W/sb/data/applications/hubos-s01.desktop
  printf '{"version":2,"saved_at":0,"entries":[{"id":1,"app_id":"hubos-s01","desktop_id":"hubos-s01.desktop","display_name":"x","position":[0,0],"size":[%s,%s],"origin":"explicit"}],"outputs":{}}' $SZ $SZ > $W/sb/session.json
  export XDG_RUNTIME_DIR=$RTD DISPLAY=:93 LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
  export __EGL_VENDOR_LIBRARY_DIRS=$A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm
  export XDG_STATE_HOME=$W/sb/st XDG_DATA_HOME=$W/sb/data
  unset WAYLAND_DISPLAY
  cd $W/sb; echo '' > c.toml
  ( ulimit -v 8000000; $BIN --backend winit --config c.toml --session-file $W/sb/session.json > log 2>&1 ) &
  sleep 7
  P=$(pgrep -n -f "driftwm-.*--session-file $W/sb/session.json")
  if [ -n "$P" ] && kill -0 $P 2>/dev/null; then echo "session entry size $SZ x $SZ: compositor alive after 7 s ($(grep VmRSS /proc/$P/status | tr -s ' ' | cut -d' ' -f2) kB)"; kill -9 $P; else echo "session entry size $SZ x $SZ: compositor DEAD: $(sed -E 's/\x1b\[[0-9;]*m//g' log | grep -a -m1 -E 'panicked|capacity|alloc' | cut -c1-120)"; fi
done
