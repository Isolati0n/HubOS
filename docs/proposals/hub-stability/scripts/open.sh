#!/bin/sh
# Exec= target of the per-machine .desktop entries: logs, then asks hubd to open the machine.
# The activation token driftwm gives us is in XDG_ACTIVATION_TOKEN (logged only whether it is set).
H=${HS_WORK}/hubstab
echo "$(date +%s.%N) open.sh $1 token_set=${XDG_ACTIVATION_TOKEN:+yes} wl=$WAYLAND_DISPLAY rt=$XDG_RUNTIME_DIR" >> /tmp/hsy-tok/open.log
${HUBD_BIN:-$H/bin/hubd} open "$1" >> /tmp/hsy-tok/open.log 2>&1
echo "$(date +%s.%N) open.sh $1 done rc=$?" >> /tmp/hsy-tok/open.log
