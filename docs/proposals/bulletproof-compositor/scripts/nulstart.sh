#!/bin/bash
# start-up and hot reload with a NUL in the keyboard layout / options string.  usage: nulstart.sh BIN
ulimit -c 0
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
BIN=$1
RTD=/tmp/bc4ns; rm -rf $RTD $W/ns; mkdir -p $RTD $W/ns; chmod 700 $RTD
export XDG_RUNTIME_DIR=$RTD DISPLAY=:93 LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
export __EGL_VENDOR_LIBRARY_DIRS=$A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm RUST_BACKTRACE=0
export XDG_STATE_HOME=$W/ns/st XDG_DATA_HOME=$W/ns/da
unset WAYLAND_DISPLAY
cd $W/ns
for kind in layout options rules; do
  printf "[input.keyboard]\n$kind = \"u\\\\u0000s\"\n" > c.toml
  $BIN --backend winit --config c.toml > log-$kind 2>&1 &
  P=$!
  sleep 4
  if kill -0 $P 2>/dev/null; then echo "start-up with NUL in $kind: compositor alive"; kill -9 $P; else wait $P; echo "start-up with NUL in $kind: compositor DEAD, status $?"; sed -E 's/\x1b\[[0-9;]*m//g' log-$kind | grep -a -A1 panicked | head -2 | sed -E 's#/tmp/claude[^ ]*scratchpad/bc4/##g' | cut -c1-200; fi
done
# hot reload
printf '[session]\nrestore_windows = true\n' > c.toml
$BIN --backend winit --config c.toml > log-reload 2>&1 &
P=$!; sleep 4
printf "[input.keyboard]\nlayout = \"u\\\\u0000s\"\n" > c.toml; sleep 3
if kill -0 $P 2>/dev/null; then echo "hot reload to a config with a NUL in layout: compositor alive"; kill -9 $P; else wait $P; echo "hot reload to a config with a NUL in layout: compositor DEAD, status $?"; sed -E 's/\x1b\[[0-9;]*m//g' log-reload | grep -a -A1 panicked | head -2 | sed -E 's#/tmp/claude[^ ]*scratchpad/bc4/##g' | cut -c1-200; fi
printf '[session]\nrestore_windows = true\n' > c.toml
$BIN --backend winit --config c.toml > log-reload2 2>&1 &
P=$!; sleep 4
printf '[keybindings]\n"alt+\\\\u0000" = "close-window"\n' > c.toml; sleep 3
if kill -0 $P 2>/dev/null; then echo "hot reload to a config with a NUL in a key binding: compositor alive"; kill -9 $P; else wait $P; echo "hot reload to a config with a NUL in a key binding: compositor DEAD, status $?"; fi
