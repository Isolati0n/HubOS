#!/bin/bash
# run the Qt6 (Debian trixie, Qt 6.8.x) test client under trixie's own glibc loader. usage: qtrun.sh APPNAME
ulimit -c 0
S=${HS_WORK}
T=$S/T/root
LIBS=$T/usr/lib/x86_64-linux-gnu
export QT_PLUGIN_PATH=$LIBS/qt6/plugins
export QT_QPA_PLATFORM=${QT_QPA_PLATFORM:-wayland}
export QT_QUICK_BACKEND=software
export XDG_DATA_DIRS=$T/usr/share
export FONTCONFIG_PATH=$T/etc/fonts
export QT_WAYLAND_DISABLE_WINDOWDECORATION=1
exec $T/usr/lib64/ld-linux-x86-64.so.2 --library-path $LIBS $S/qtc/qtc "$@"
