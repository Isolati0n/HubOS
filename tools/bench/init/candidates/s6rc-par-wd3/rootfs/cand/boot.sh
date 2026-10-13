# s6-svscan is PID 1 with a scan directory that holds only a "stage2" service; stage2 runs s6-rc-init and brings up the graph.
echo "CH CAND s6rc pid1=s6-svscan + s6-rc"
mkdir -p /run/service/.s6-svscan /run/service/stage2
printf '#!/bin/sh\necho "CH s6-svscan finished; rebooting"\nreboot -f\n' > /run/service/.s6-svscan/finish
printf '#!/bin/sh\necho "CH s6-svscan crashed; rebooting"\nreboot -f\n' > /run/service/.s6-svscan/crash
cat > /run/service/stage2/run <<'EOS'
#!/bin/sh
exec 2>&1
[ -d /run/s6-rc ] || s6-rc-init -c /etc/s6-rc/compiled -l /run/s6-rc /run/service || { ls -la /run/service /run/s6-rc /run/s6-rc/servicedirs; which s6-supervise; ls /opt/s6/bin; }
s6-rc -l /run/s6-rc -u change top
echo "CH S6RC-UP up=$(cut -d' ' -f1 /proc/uptime)"
exec sleep 2147483647
EOS
chmod +x /run/service/.s6-svscan/finish /run/service/.s6-svscan/crash /run/service/stage2/run
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec s6-svscan /run/service
