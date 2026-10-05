echo "CH CAND dinit pid1=dinit"
echo "CH SUP-START up=$(cut -d' ' -f1 /proc/uptime)"
exec /opt/dinit/bin/dinit -d /etc/dinit.d boot
