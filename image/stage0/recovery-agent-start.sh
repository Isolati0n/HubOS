#!/bin/sh
# Started by recovery-init after DHCP, ONLY in the TEST recovery kernel that carries the recovery agent
# (docs/proposals/recovery-and-out-of-band.md). Serves /v1/status and the signed requests on port 8480 of the recovery
# kernel's address; the management public keys are in /etc/hubos/mgmt. The real work is done by hubos-ctl.
setsid /usr/sbin/recovery-agent -backend hubos -listen :8480 -keys /etc/hubos/mgmt >/run/recovery-agent.log 2>&1 &
echo "STAGE0-RECOVERY: recovery agent started on port 8480 (TEST kernel)"
