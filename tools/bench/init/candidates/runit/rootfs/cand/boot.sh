echo "CH CAND runit pid1=runit stage2=runsvdir"
cp -a /etc/sv /run/service
touch /run/runit.reboot; chmod +x /run/runit.reboot   # if stage 2 ever returns, stage 3 ends in a reboot (like the s6 image's finish script)
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec /sbin/runit
