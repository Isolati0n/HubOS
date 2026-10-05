#!/bin/bash
# Sourced by every script. Sets up the private tool folder. Nothing is installed on the machine.
# BENCH_TMP is the only place outside the repository that this harness writes to.
BENCH_TMP=${BENCH_TMP:-/tmp/bench}
BENCH_ROOT=$BENCH_TMP/a/root          # unpacked .deb files (dpkg -x)
BENCH_PREFIX=$BENCH_TMP/prefix        # our own builds (wayvnc, neatvnc, aml, wlvncc)
BENCH_SRC=$BENCH_TMP/src
BENCH_RUN=${BENCH_RUN:-$BENCH_TMP/run}   # XDG_RUNTIME_DIR for all compositors (mode 0700)
BENCH_OUT=${BENCH_OUT:-$BENCH_TMP/out}   # raw scratch output (screenshots, logs)
BENCH_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
LIBS=$BENCH_ROOT/usr/lib/x86_64-linux-gnu:$BENCH_ROOT/lib/x86_64-linux-gnu:$BENCH_ROOT/usr/lib/x86_64-linux-gnu/weston:$BENCH_PREFIX/lib:$BENCH_PREFIX/lib/x86_64-linux-gnu
export PATH=$BENCH_PREFIX/bin:$BENCH_ROOT/usr/bin:$BENCH_ROOT/usr/sbin:$BENCH_TMP/py/bin:$BENCH_TMP/go/bin:$PATH
export LD_LIBRARY_PATH=$LIBS${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}
export XDG_DATA_DIRS=$BENCH_ROOT/usr/share:$BENCH_PREFIX/share:/usr/local/share:/usr/share
export PYTHONPATH=$BENCH_TMP/py${PYTHONPATH:+:$PYTHONPATH}
if [ ! -f "$BENCH_TMP/fonts.conf" ]; then   # fontconfig must find the unpacked fonts
  printf '<?xml version="1.0"?><!DOCTYPE fontconfig SYSTEM "fonts.dtd"><fontconfig><dir>%s/usr/share/fonts</dir><cachedir>%s/fontcache</cachedir></fontconfig>\n' "$BENCH_ROOT" "$BENCH_TMP" > "$BENCH_TMP/fonts.conf"
fi
export FONTCONFIG_FILE=$BENCH_TMP/fonts.conf
export XDG_RUNTIME_DIR=$BENCH_RUN
export WLR_RENDERER=pixman WLR_BACKENDS=headless WLR_LIBINPUT_NO_DEVICES=1
mkdir -p -m 0700 "$BENCH_RUN" "$BENCH_OUT"
