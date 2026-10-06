#!/bin/bash
B=${BC_WORK:?set BC_WORK to your work folder}
for v in subpos_2147483647 subpos_1073741824 subpos_16777216 popupoff_2147483647 popupoff_1073741824 popupoff_16777216 layermargin_2147483647 layermargin_1073741824 layermargin_16777216; do
  bash $B/reprorun.sh $1 $2-$v python3 $B/repro_damage.py SOCK DPID $v 2>&1 | grep -E "RESULT|smashing|panicked" | tr '\n' ' '; echo
done
