echo "CH CAND b-go pid1=s6-svscan, supervisor=brain(go, direct)"
mkdir -p /run/service/.s6-svscan /run/service/brain /run/service/guard
printf '#!/bin/sh\necho "CH s6-svscan finished; rebooting"\nreboot -f\n' > /run/service/.s6-svscan/finish
printf '#!/bin/sh\necho "CH s6-svscan crashed; rebooting"\nreboot -f\n' > /run/service/.s6-svscan/crash
printf '#!/bin/sh\nexec 2>&1\nexec brain -mode direct\n' > /run/service/brain/run
printf '#!/bin/sh\nexec 2>&1\necho -1000 > /proc/self/oom_score_adj\nexec hubsim guard /dev/watchdog\n' > /run/service/guard/run
chmod +x /run/service/.s6-svscan/finish /run/service/.s6-svscan/crash /run/service/brain/run /run/service/guard/run
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec s6-svscan /run/service
