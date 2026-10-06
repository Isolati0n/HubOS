#!/bin/bash
# usage: xvfb.sh DISPLAY WxHxD   (Xvfb in a private mount namespace where /usr/bin also holds xkbcomp; nothing is installed)
ulimit -c 0
W=${BC_WORK:?set BC_WORK to your work folder}
A=$W/A
exec unshare -m --propagation private bash -c "
mount -t overlay overlay -o lowerdir=$A/root/usr/bin:/usr/bin /usr/bin || exit 90
export LD_LIBRARY_PATH=$A/root/usr/lib/x86_64-linux-gnu
exec nice -n 15 $A/root/usr/bin/Xvfb $1 -screen 0 $2 -nolisten tcp -xkbdir /usr/share/X11/xkb
"
