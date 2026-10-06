#!/bin/bash
# what does the compositor do at start-up and on hot reload with a config that has one unknown field?
ulimit -c 0
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
RTD=/tmp/bc4cs; rm -rf $RTD $W/cs; mkdir -p $RTD $W/cs; chmod 700 $RTD
export XDG_RUNTIME_DIR=$RTD DISPLAY=:93 LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
export __EGL_VENDOR_LIBRARY_DIRS=$A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm
export XDG_STATE_HOME=$W/cs/st XDG_DATA_HOME=$W/cs/da
unset WAYLAND_DISPLAY
cd $W/cs
printf '[session]\nrestore_windows = true\n\n[effects]\nblur = false\n' > c.toml
$W/bin/driftwm-p20 --backend winit --config c.toml > log1 2>&1 &
P=$!
sleep 3
echo "--- start-up with an unknown field in [effects]: log lines mentioning config:"
sed -E 's/\x1b\[[0-9;]*m//g' log1 | grep -i -E "config|unknown|default" | cut -c1-260 | head -6
printf '[session]\nrestore_windows = true\n' > c.toml; sleep 2
echo "--- hot reload to a good config:"
sed -E 's/\x1b\[[0-9;]*m//g' log1 | grep -i -E "reload|config" | tail -3 | cut -c1-260
printf '[session]\nrestore_windows = true\n\n[effects]\nblur = false\n' > c.toml; sleep 2
echo "--- hot reload back to the config with the unknown field:"
sed -E 's/\x1b\[[0-9;]*m//g' log1 | grep -i -E "reload|config" | tail -3 | cut -c1-260
kill -9 $P
