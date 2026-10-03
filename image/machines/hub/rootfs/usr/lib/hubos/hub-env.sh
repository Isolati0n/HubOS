# Sourced by the hub's service run scripts (they start as root): makes the desktop user's runtime folders (idempotent) and
# sets HUBENV, the environment the desktop programs get. driftwm's session state lives in RAM (XDG_STATE_HOME=/run/hub/state):
# windows are never restored after a reboot. The machine's own settings for the desktop (for example LIBGL_ALWAYS_SOFTWARE=1
# in a virtual machine) come from /config/hubos/desktop.env, one NAME=VALUE per line, outside the image.
mkdir -p /run/dw /run/hub/state /run/hubos /dev/pts /dev/shm
chown hub:hub /run/dw /run/hub /run/hub/state /run/hubos 2>/dev/null
chmod 700 /run/dw /run/hubos
grep -q ' /dev/pts ' /proc/mounts || mount -t devpts devpts /dev/pts
grep -q ' /dev/shm ' /proc/mounts || mount -t tmpfs tmpfs /dev/shm
HUBENV="XDG_RUNTIME_DIR=/run/dw XDG_STATE_HOME=/run/hub/state HOME=/run/hub XDG_SEAT=seat0 LIBSEAT_BACKEND=seatd WAYLAND_DISPLAY=wayland-1 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/dw/bus LC_ALL=C.UTF-8"
[ -f /config/hubos/desktop.env ] && HUBENV="$HUBENV $(grep -E '^[A-Za-z_][A-Za-z0-9_]*=' /config/hubos/desktop.env | tr '\n' ' ')"
export HUBENV
