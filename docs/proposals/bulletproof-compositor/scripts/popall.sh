#!/bin/bash
# usage: popall.sh BIN LABELPREFIX
B=${BC_WORK:?set BC_WORK to your work folder}
for c in self pair nullparent; do
  bash $B/reprorun.sh $1 $2-$c python3 $B/repro_popup.py SOCK $c 2>&1 | tail -20
done
