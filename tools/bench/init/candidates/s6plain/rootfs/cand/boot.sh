# s6 plain: copy the service directories to the (RAM) scan directory and become s6-svscan, as image/rootfs/usr/sbin/init does.
echo "CH CAND s6plain pid1=s6-svscan"
cp -a /etc/s6/sv /run/service
mkdir -p /run/service/.s6-svscan
printf '#!/bin/sh\necho "CH s6-svscan finished; rebooting"\nreboot -f\n' > /run/service/.s6-svscan/finish
printf '#!/bin/sh\necho "CH s6-svscan crashed; rebooting"\nreboot -f\n' > /run/service/.s6-svscan/crash
chmod +x /run/service/.s6-svscan/finish /run/service/.s6-svscan/crash
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec s6-svscan /run/service
