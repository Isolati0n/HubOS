#!/bin/bash
B=${BC_WORK:?set BC_WORK to your work folder}
for v in damage_imax damage_neg damage_imin damage_buffer_imax damage_buffer_neg damage_big_offset damage_zero scale_big scale_zero scale_neg viewport_dst_big viewport_src_big attach_offset_big; do
  bash $B/reprorun.sh $1 $2-$v python3 $B/repro_damage.py SOCK DPID $v 2>&1 | grep -E "RESULT|->|panicked|smashing" | tr '\n' ' '; echo
done
