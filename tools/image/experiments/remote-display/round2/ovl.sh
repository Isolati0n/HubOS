#!/bin/sh
# run a command in a private mount namespace where /usr/bin also shows the unpacked xkbcomp (vanishes with the process)
exec unshare -m sh -c 'mount -t overlay overlay -o lowerdir=/tmp/r3-x/root/usr/bin:/tmp/r3-a/root/usr/bin:/usr/bin /usr/bin && exec "$@"' sh "$@"
