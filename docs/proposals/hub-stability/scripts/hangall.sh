ulimit -c 0
D=${HS_WORK}
cd /tmp
for m in sigstop busyloop deadlock panic; do
  rm -rf /tmp/hs-hang-$m /tmp/hsy /tmp/hsy-tok; mkdir -p /tmp/hsy-tok
  echo "=== $m"
  EXEC_MODE=foot HANG_WINDOW=30 python3 $D/hs/planA.py /tmp/hs-hang-$m $m $D/hubstab/bin/driftwm-patched all 2>&1 | grep -E "FAILURE|probe|HUNG|evidence|hang detection|exited|all viewers|answers IPC|stand-ins|relaunch:|geometry" | cut -c1-330 | awk 'NR<=6 || !/probe:/'
done
