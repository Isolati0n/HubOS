#!/bin/bash
# Builds the two small C helpers into $BENCH_PREFIX/bin: benchapp (node test window) and pixelwatch (hub pixel sampler).
. "$(dirname "$0")/../env.sh"
set -e
P=$BENCH_ROOT/usr/share/wayland-protocols; T=$BENCH_TMP/gen; mkdir -p $T $BENCH_PREFIX/bin
SC=$BENCH_SRC/wayvnc/protocols/wlr-screencopy-unstable-v1.xml
export PKG_CONFIG_PATH=$BENCH_ROOT/usr/lib/x86_64-linux-gnu/pkgconfig
export CPATH=$BENCH_ROOT/usr/include:$BENCH_ROOT/usr/include/x86_64-linux-gnu
export LIBRARY_PATH=$BENCH_ROOT/usr/lib/x86_64-linux-gnu
wayland-scanner client-header $P/stable/xdg-shell/xdg-shell.xml $T/xdg-shell-client-protocol.h
wayland-scanner private-code  $P/stable/xdg-shell/xdg-shell.xml $T/xdg-shell-protocol.c
wayland-scanner client-header $SC $T/wlr-screencopy-unstable-v1-client-protocol.h
wayland-scanner private-code  $SC $T/wlr-screencopy-unstable-v1-protocol.c
VP=$BENCH_SRC/wayvnc/protocols/wlr-virtual-pointer-unstable-v1.xml
wayland-scanner client-header $VP $T/wlr-virtual-pointer-unstable-v1-client-protocol.h
wayland-scanner private-code  $VP $T/wlr-virtual-pointer-unstable-v1-protocol.c
VK=$BENCH_SRC/wayvnc/protocols/virtual-keyboard-unstable-v1.xml
wayland-scanner client-header $VK $T/virtual-keyboard-unstable-v1-client-protocol.h
wayland-scanner private-code  $VK $T/virtual-keyboard-unstable-v1-protocol.c
D=$BENCH_DIR
gcc -O2 -Wall -I$T -o $BENCH_PREFIX/bin/hubclick $D/metrics/hubclick.c $T/wlr-virtual-pointer-unstable-v1-protocol.c $T/virtual-keyboard-unstable-v1-protocol.c -lwayland-client -lxkbcommon
gcc -O2 -Wall -I$T -o $BENCH_PREFIX/bin/benchapp $D/scenes/benchapp.c $T/xdg-shell-protocol.c -lwayland-client
gcc -O2 -Wall -I$T -o $BENCH_PREFIX/bin/pixelwatch $D/metrics/pixelwatch.c $T/wlr-screencopy-unstable-v1-protocol.c -lwayland-client
cd $BENCH_DIR/../../.. && go build -o $BENCH_PREFIX/bin/ssim ./tools/bench/remote-display/ssim
