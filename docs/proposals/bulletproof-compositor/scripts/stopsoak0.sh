#!/bin/bash
# stops soak chunk 0 (the single long run) by explicit PIDs only
ulimit -c 0
kill -TERM 3317 23412
sleep 1
kill -KILL 27293 4984 5954 7375 10477 12622 14048 14439 14497 14703 16008 22549 22745 22838 22995 23204 23504 23713 23868 24213 32172 2>/dev/null
sleep 1
kill -TERM 7469 2>/dev/null
sleep 1
echo "left in session 3317:"; ps -eo pid,sid,comm | awk '$2==3317'
