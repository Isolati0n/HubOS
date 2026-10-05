#!/bin/bash
. /tmp/claude-0/-home-user-HubOS/19ba1693-e641-5e2a-a3db-c8037607fada/scratchpad/ib/env.sh
export PATH=$W/hostbin:$T/usr/lib/elixir/bin:$PATH
export LD_LIBRARY_PATH=$T/usr/lib/x86_64-linux-gnu:$T/lib/x86_64-linux-gnu
export ELIXIR_ERL_OPTIONS="+S 1:1"
"$@"
