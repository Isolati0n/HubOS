#!/bin/sh
# Started by recovery-init after DHCP, ONLY in the TEST recovery kernel that carries the recovery agent
# (docs/proposals/recovery-and-out-of-band.md). Serves /v1/status and the other requests on port 8480 of the recovery
# kernel's address. Requests are NOT signed; the install request is checked by hubos-ctl, which refuses any bundle not signed
# by the update key (the image signature). The real work is done by hubos-ctl.
#
# The agent is SUPERVISED by a plain shell restart loop (no s6 in the recovery kernel):
#  - when the agent exits for any reason, a line is logged (to /run/recovery-agent.log and the console) and it is started again
#    ONE SECOND later;
#  - the loop does not hide a crash loop: a run that lasted less than 30 s counts as a "short run"; after 5 SHORT RUNS IN A ROW
#    the loop gives up (logs "giving up") and leaves the agent dead, so the machine shows "recovery, agent not answering" instead of
#    restarting forever. A run of 30 s or more resets the count.
LOG=/run/recovery-agent.log
supervise() {
  short=0
  while :; do
    start=$(date +%s)
    /usr/sbin/recovery-agent -backend hubos -listen :8480 >>$LOG 2>&1
    rc=$?
    ran=$(( $(date +%s) - start ))
    if [ "$ran" -lt 30 ]; then short=$((short + 1)); else short=0; fi
    msg="recovery-agent-supervisor: the agent exited with status $rc after $ran s (short runs in a row: $short of 5)"
    echo "$msg" >>$LOG; echo "$msg" >/dev/console 2>/dev/null
    if [ "$short" -ge 5 ]; then
      msg="recovery-agent-supervisor: 5 short runs in a row: giving up (the agent stays down; the shell keeps working)"
      echo "$msg" >>$LOG; echo "$msg" >/dev/console 2>/dev/null
      return
    fi
    sleep 1
    echo "recovery-agent-supervisor: starting the agent again" >>$LOG
  done
}
supervise &
echo "STAGE0-RECOVERY: recovery agent started on port 8480 (TEST kernel, supervised by a restart loop)"
