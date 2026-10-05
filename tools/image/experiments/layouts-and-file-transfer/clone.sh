#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# fetch driftwm and check out the commit the project pins (HUB-OS.md); stop if that is not what we have
ulimit -c 0
PIN=352333a8fa1b22171492d4b71a54102045c9a19d
cd $LFT
rm -rf src
GIT_LFS_SKIP_SMUDGE=1 git clone -q --depth 50 https://github.com/malbiruk/driftwm src 2>&1 | tail -3
cd src
git checkout -q $PIN 2>/dev/null || { git fetch -q --depth 500 origin $PIN && git checkout -q $PIN; }
[ "$(git rev-parse HEAD)" = "$PIN" ] || { echo "driftwm is not at the pinned commit" >&2; exit 1; }
echo "driftwm at $PIN"
