#!/bin/bash
for i in a b; do for f in wp pw dbus; do [ -f /tmp/p4-$i/$f.pid ] && kill $(cat /tmp/p4-$i/$f.pid) 2>/dev/null; done; done
sleep 1
