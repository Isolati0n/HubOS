#!/bin/bash
# private apt state; nothing installed
ulimit -c 0
W=/tmp/claude-0/-home-user-HubOS/19ba1693-e641-5e2a-a3db-c8037607fada/scratchpad/ib
mkdir -p $W/apt/lists/partial $W/apt/cache/archives/partial $W/debs $W/root
APT() { apt-get -o Dir::State::lists=$W/apt/lists -o Dir::Cache=$W/apt/cache -o APT::Sandbox::User=root "$@"; }
CACHE() { apt-cache -o Dir::State::lists=$W/apt/lists -o Dir::Cache=$W/apt/cache "$@"; }
