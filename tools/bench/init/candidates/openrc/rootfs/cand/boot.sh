echo "CH CAND openrc pid1=openrc-init"
export LD_LIBRARY_PATH=/lib/x86_64-linux-gnu:/lib
# the guard is started here, not by OpenRC: supervise-daemon sets itself as a child subreaper (BELIEVED), which would hide an unreaped orphan from the PID 1 check
echo -1000 > /proc/self/oom_score_adj
/bin/hubsim guard /dev/watchdog &
echo 0 > /proc/self/oom_score_adj
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec /sbin/openrc-init
