#!/bin/bash
B=${BC_WORK:?set BC_WORK to your work folder}
mkdir -p $B/resu
{
echo "## configwipe.py on driftwm-p25: 8 live windows, restore_windows=true; an odd config while a window changes"
rm -rf $B/cw
bash $B/py.sh $B/configwipe.py $B/cw $B/bin/driftwm-p25 8 2>&1
echo
echo "## cfgstart.sh (unknown field in [effects] at start-up and at hot reload), driftwm-p20"
bash $B/cfgstart.sh 2>&1 | cut -c1-230
echo
echo "## nulcfg.sh / nulstart.sh on the pristine build and on p25 are in verify-pristine.txt and verify-p25.txt"
} > $B/resu/config-tests.txt
