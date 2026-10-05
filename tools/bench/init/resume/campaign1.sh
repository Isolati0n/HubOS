#!/bin/bash
# campaign 1: boot x5, pid1crash x2, pid1wedge x1, trial x3, faults (reps 5) for every candidate; one VM at a time
. /tmp/claude-0/-home-user-HubOS/19ba1693-e641-5e2a-a3db-c8037607fada/scratchpad/ib/env.sh
cd $REPO
export W T REPO
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $W/hubsim ./tools/bench/init/hubsim || exit 1
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $W/brain-go ./tools/bench/init/prototypes/go || exit 1
mkdir -p $W/runs/c1
CANDS=${CANDS:-"s6plain s6rc runit dinit openrc c-go c-elixir b-go"}
for c in $CANDS; do
  bash tools/bench/init/mkrootfs.sh $c > $W/runs/build-$c.log 2>&1 || { echo "BUILD FAILED $c"; continue; }
  for i in 1 2 3 4 5; do python3 tools/bench/init/run-qemu.py $c boot --timeout 150 --seed $((100+i)) --out $W/runs/c1/$c-boot-$i.log; done
  for i in 1 2; do python3 tools/bench/init/run-qemu.py $c pid1crash --timeout 300 --seed $((110+i)) --out $W/runs/c1/$c-pid1crash-$i.log; done
  python3 tools/bench/init/run-qemu.py $c pid1wedge --timeout 400 --seed 120 --out $W/runs/c1/$c-pid1wedge-1.log
  python3 tools/bench/init/run-qemu.py $c trial --trial healthy --timeout 250 --seed 130 --out $W/runs/c1/$c-trial-healthy.log
  python3 tools/bench/init/run-qemu.py $c trial --trial crash --preflag driftwm.crash --timeout 250 --seed 131 --out $W/runs/c1/$c-trial-crash.log
  python3 tools/bench/init/run-qemu.py $c trial --trial hang --timeout 250 --seed 132 --out $W/runs/c1/$c-trial-hang.log
  python3 tools/bench/init/run-qemu.py $c faults --reps 5 --timeout 3600 --seed 200 --out $W/runs/c1/$c-faults-1.log
  case $c in c-go|c-elixir|b-go) python3 tools/bench/init/run-qemu.py $c bug --timeout 400 --seed 140 --out $W/runs/c1/$c-bug-1.log;; esac
done
echo CAMPAIGN1-DONE
