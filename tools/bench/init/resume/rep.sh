#!/bin/bash
. /tmp/claude-0/-home-user-HubOS/19ba1693-e641-5e2a-a3db-c8037607fada/scratchpad/ib/env.sh
cd $REPO
python3 tools/bench/init/report.py "$@"
