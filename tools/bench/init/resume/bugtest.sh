#!/bin/bash
# bugtest.sh cand seed : the policy-bug suite (needs the brain candidates)
. /tmp/claude-0/-home-user-HubOS/19ba1693-e641-5e2a-a3db-c8037607fada/scratchpad/ib/env.sh
cd $REPO
export W T REPO
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $W/hubsim ./tools/bench/init/hubsim || exit 1
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $W/brain-go ./tools/bench/init/prototypes/go || exit 1
mkdir -p $W/runs/c1
bash tools/bench/init/mkrootfs.sh $1 >/dev/null 2>&1 || exit 1
python3 tools/bench/init/run-qemu.py $1 bug --timeout 400 --seed $2 --out $W/runs/c1/$1-bug-1.log
grep -E "policy-bug|UNREC|BOOT-OK" $W/runs/c1/$1-bug-1.log | cut -c1-250
