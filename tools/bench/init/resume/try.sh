#!/bin/bash
# try.sh CAND SUITE [extra run-qemu args]: build hubsim and the Go brain, build the rootfs, run once
. /tmp/claude-0/-home-user-HubOS/19ba1693-e641-5e2a-a3db-c8037607fada/scratchpad/ib/env.sh
cd $REPO
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $W/hubsim ./tools/bench/init/hubsim || exit 1
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $W/brain-go ./tools/bench/init/prototypes/go || exit 1
export W T REPO
bash tools/bench/init/mkrootfs.sh $1 || exit 1
c=$1; s=$2; shift 2
python3 tools/bench/init/run-qemu.py $c $s "$@"
