: "${LFT:?set LFT to the work folder (see README.md)}"
# source me. Nested-driftwm test environment (parent = headless sway, because Xvfb needs /usr/bin/xkbcomp, which is absent)
. $LFT/env.sh
T=$W/t
export XDG_RUNTIME_DIR=/tmp/dwx XDG_STATE_HOME=$T/state XDG_DATA_HOME=$T/data XDG_DATA_DIRS=$T/data
mkdir -p $XDG_RUNTIME_DIR; chmod 700 $XDG_RUNTIME_DIR
DW=${DWBIN:-$W/target/debug/driftwm}
dw() { $DW "$@"; }
start_sway() {
  env -u WAYLAND_DISPLAY WLR_BACKENDS=headless WLR_RENDERER=pixman WLR_LIBINPUT_NO_DEVICES=1 WLR_HEADLESS_OUTPUTS=1 \
    sway -c $T/sway.conf > $T/sway.log 2>&1 &
  echo $! > $T/sway.pid; sleep 3
  PARENT=$(ls $XDG_RUNTIME_DIR | grep -E '^wayland-[0-9]+$' | head -1); export PARENT
}
clean_dw() { sed -E 's/\x1b\[[0-9;]*m//g' $T/dw.log; }
# start_dw CONFIG [driftwm args]: nested driftwm on the sway parent; WAYLAND_DISPLAY then points at it
start_dw() { cfg=$1; shift; [ -f $T/dw.pid ] && kill -0 $(cat $T/dw.pid) 2>/dev/null && kill $(cat $T/dw.pid) 2>/dev/null; sleep 1; env WAYLAND_DISPLAY=$PARENT RUST_LOG=info $DW --backend winit --config $cfg "$@" > $T/dw.log 2>&1 & echo $! > $T/dw.pid; sleep 4
  export WAYLAND_DISPLAY=$(clean_dw | grep -oE 'Listening on WAYLAND_DISPLAY=[a-z0-9-]+' | head -1 | cut -d= -f2); }
