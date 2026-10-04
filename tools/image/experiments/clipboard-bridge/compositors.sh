#!/bin/sh
# Starts or stops two headless sway compositors, "hub" and "node", each with its
# own XDG_RUNTIME_DIR (/tmp/nh-hub and /tmp/nh-node). Nothing is installed:
# NH_ROOT is a directory where the .deb files were unpacked with dpkg -x
# (sway, libwlroots12t64, wl-clipboard and their libraries; see README.md).
# usage: compositors.sh start|stop
NH_ROOT=${NH_ROOT:-/tmp/nh-root}
export LD_LIBRARY_PATH=$NH_ROOT/usr/lib/x86_64-linux-gnu PATH=$NH_ROOT/usr/bin:$PATH
case "$1" in
start)
	echo 'output HEADLESS-1 resolution 800x600' > "$NH_ROOT/sway.conf"
	for n in hub node; do
		mkdir -p /tmp/nh-$n && chmod 700 /tmp/nh-$n
		XDG_RUNTIME_DIR=/tmp/nh-$n WLR_BACKENDS=headless WLR_RENDERER=pixman \
			WLR_LIBINPUT_NO_DEVICES=1 nice -n 15 sway -c "$NH_ROOT/sway.conf" \
			> /tmp/nh-$n/sway.log 2>&1 &
		echo $! > /tmp/nh-$n/pid
	done
	sleep 3
	ls /tmp/nh-hub/wayland-1 /tmp/nh-node/wayland-1
	;;
stop)
	for n in hub node; do
		[ -f /tmp/nh-$n/pid ] && kill "$(cat /tmp/nh-$n/pid)" 2>/dev/null
	done
	;;
*) echo "usage: $0 start|stop"; exit 2 ;;
esac
