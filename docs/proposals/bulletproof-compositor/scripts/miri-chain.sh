#!/bin/bash
B=${BC_WORK:?set BC_WORK to your work folder}
until grep -q "^b2: total" $B/miri-b2.tsv 2>/dev/null; do sleep 20; done
bash $B/miri-each.sh b3 config:: 2>&1 | tail -2
