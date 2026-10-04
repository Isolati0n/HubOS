#!/bin/bash
# start.sh MEDIA LATENCY_MS : two PipeWire instances (a, b) with their own runtime dirs, WirePlumber and a session bus each.
# MEDIA is audio (raw) or opus. Everything under /tmp/p4-a and /tmp/p4-b. Run stop.sh to remove.
D=$(dirname "$(readlink -f "$0")"); MEDIA=${1:-audio}; LAT=${2:-100}
if [ -z "$INNER" ]; then   # retry: the RTP stream sometimes starts before the sink exists ("no target node available")
  for try in 1 2 3 4; do
    INNER=1 "$0" "$MEDIA" "$LAT"
    XDG_RUNTIME_DIR=/tmp/p4-a pw-link -l 2>/dev/null | grep -q "|<- node-null" && XDG_RUNTIME_DIR=/tmp/p4-b pw-link -l 2>/dev/null | grep -q "|<- rtp-from-a-at-hub" && exit 0
    echo "start attempt $try: links missing, restarting" >&2
  done
  exit 1
fi
"$D/stop.sh" >/dev/null 2>&1
for i in a b; do
  R=/tmp/p4-$i; mkdir -p $R/cfg/pipewire/pipewire.conf.d; chmod 700 $R
  f=node-a.conf; [ $i = b ] && f=hub-b.conf
  sed "s/RTPMEDIA/$MEDIA/; s/LATMS/$LAT/" "$D/$f" > $R/cfg/pipewire/pipewire.conf.d/10-test.conf
  dbus-daemon --session --address=unix:path=$R/bus --nofork --nopidfile >$R/dbus.log 2>&1 &
  echo $! > $R/dbus.pid
done
sleep 1
for i in a b; do R=/tmp/p4-$i
  export DBUS_SESSION_BUS_ADDRESS=unix:path=$R/bus XDG_RUNTIME_DIR=$R PIPEWIRE_RUNTIME_DIR=$R XDG_CONFIG_HOME=$R/cfg
  nice -n 15 pipewire >$R/pw.log 2>&1 & echo $! > $R/pw.pid
done
sleep 2
for i in a b; do R=/tmp/p4-$i
  export DBUS_SESSION_BUS_ADDRESS=unix:path=$R/bus XDG_RUNTIME_DIR=$R PIPEWIRE_RUNTIME_DIR=$R XDG_CONFIG_HOME=$R/cfg
  nice -n 15 wireplumber >$R/wp.log 2>&1 & echo $! > $R/wp.pid
done
sleep 3
