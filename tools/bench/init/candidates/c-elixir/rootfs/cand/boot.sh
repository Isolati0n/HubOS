echo "CH CAND c-elixir pid1=s6-svscan + s6-rc + brain(elixir)"
mkdir -p /run/service/.s6-svscan /run/service/stage2 /run/service/brain
printf '#!/bin/sh\necho "CH s6-svscan finished; rebooting"\nreboot -f\n' > /run/service/.s6-svscan/finish
printf '#!/bin/sh\necho "CH s6-svscan crashed; rebooting"\nreboot -f\n' > /run/service/.s6-svscan/crash
touch /run/service/brain/down
cp /opt/brain/run /run/service/brain/run
mkdir -p /run/service/brain/log
printf '#!/bin/sh\nmkdir -p /var/log/brain\nexec s6-log -b n5 s100000 /var/log/brain\n' > /run/service/brain/log/run
cat > /run/service/stage2/run <<'EOS'
#!/bin/sh
exec 2>&1
[ -d /run/s6-rc ] || s6-rc-init -c /etc/s6-rc/compiled -l /run/s6-rc /run/service
s6-rc -l /run/s6-rc -u change top
echo "CH S6RC-UP up=$(cut -d' ' -f1 /proc/uptime)"
s6-svc -u /run/service/brain
exec sleep 2147483647
EOS
chmod +x /run/service/.s6-svscan/finish /run/service/.s6-svscan/crash /run/service/stage2/run /run/service/brain/run /run/service/brain/log/run
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec s6-svscan /run/service
