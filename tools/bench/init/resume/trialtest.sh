#!/bin/bash
# trialtest.sh cand scenario seed
. /tmp/claude-0/-home-user-HubOS/19ba1693-e641-5e2a-a3db-c8037607fada/scratchpad/ib/env.sh
cd $REPO
export W T REPO
c=$1; scn=$2; seed=$3
pre=""
[ "$scn" = crash ] && pre="driftwm.crash"
[ -f $W/rf/$c.cpio.gz ] || bash tools/bench/init/mkrootfs.sh $c >/dev/null 2>&1
python3 tools/bench/init/run-qemu.py $c trial --trial $scn --preflag "$pre" --timeout 300 --seed $seed --out $W/runs/$c-trial-$scn-$seed.log
grep -E "TRIAL|STATUS|BOOT-OK" $W/runs/$c-trial-$scn-$seed.log | cut -c1-200 | head -14
