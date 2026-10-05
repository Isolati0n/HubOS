#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test: a "file clipboard" as text/uri-list on a Wayland compositor with data-control (sway headless standing in for a node).
. $LFT/env.sh
export XDG_RUNTIME_DIR=/tmp/dwx WAYLAND_DISPLAY=wayland-1
cd /tmp
printf 'file:///srv/share/a%%20b.txt\r\nfile:///srv/share/proj\r\n' | wl-copy --type text/uri-list
sleep 0.5
echo "types on the clipboard:"; wl-paste --list-types
echo "--- read as uri-list:"; wl-paste -n --type text/uri-list | od -c | head -4
echo "--- read as plain text (what wayvnc asks for):"; wl-paste -n --type 'text/plain;charset=utf-8'; echo "exit=$?"
echo "--- read as 'text' (hub clipboard bridge's wl-paste --type text):"; wl-paste -n --type text; echo "exit=$?"
echo "--- set plain text afterwards: uri-list is replaced:"; echo -n "plain words" | wl-copy --type 'text/plain;charset=utf-8'; sleep 0.3; wl-paste --list-types | tr '\n' ' '; echo
wl-copy --clear
