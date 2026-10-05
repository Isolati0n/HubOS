#!/bin/bash
# peek.sh LOG SECONDS_TO_WAIT [N]: wait, then show the interesting lines
sleep ${2:-0}
grep -E "CH t=.* (EV|UNREC|STUCK|STORM|DEPSTARTS|BOOT-FAILED|operator)|brain:|launch|H# end|vm exit|panic|Attempted" "$1" | cut -c1-230 | tail -${3:-40}
