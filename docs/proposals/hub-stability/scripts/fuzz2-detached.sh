#!/bin/bash
ulimit -c 0
D=${HS_WORK}
cat > /tmp/hs-fzseq2.sh <<EOF
export FUZZ_SKIP=wl_shm_pool.resize
bash $D/hs/fuzzloop.sh $D/hubstab/bin/driftwm-pristine release2 30 35 120
export FUZZ_SKIP=
export HS_CATCH_PANICS=1
bash $D/hs/fuzzloop.sh $D/hubstab/bin/driftwm-debug-patched catchdebug 50 55 60
EOF
setsid nohup bash /tmp/hs-fzseq2.sh > /dev/null 2>&1 < /dev/null &
sleep 1
echo started
