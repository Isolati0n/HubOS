#!/bin/bash
# runall.sh SUITE "extra args" cand...   (one VM at a time)
. /tmp/claude-0/-home-user-HubOS/19ba1693-e641-5e2a-a3db-c8037607fada/scratchpad/ib/env.sh
cd $REPO
export W T REPO
suite=$1; extra=$2; shift 2
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $W/hubsim ./tools/bench/init/hubsim || exit 1
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $W/brain-go ./tools/bench/init/prototypes/go || exit 1
for c in "$@"; do
  bash tools/bench/init/mkrootfs.sh $c > $W/runs/build-$c.log 2>&1 || { echo "BUILD FAILED $c"; continue; }
  python3 tools/bench/init/run-qemu.py $c $suite $extra
done
echo ALLDONE
