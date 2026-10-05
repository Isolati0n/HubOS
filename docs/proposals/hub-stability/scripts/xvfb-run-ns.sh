#!/bin/bash
# Start Xvfb in a private mount namespace where /usr/bin also contains xkbcomp (nothing is installed on the machine).
# usage: xvfb-run-ns.sh DISPLAY WxHxD
ulimit -c 0
S=${HS_WORK}
A=$S/A
exec unshare -m --propagation private bash -c "
mount -t overlay overlay -o lowerdir=$A/root/usr/bin:/usr/bin /usr/bin || exit 90
export LD_LIBRARY_PATH=$A/root/usr/lib/x86_64-linux-gnu
exec $A/root/usr/bin/Xvfb $1 -screen 0 $2 -nolisten tcp -xkbdir /usr/share/X11/xkb
"
