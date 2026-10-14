# dinit 0.21.0 built from the Alpine source mirror (static, g++), see README.md.
mkdir -p "$R/opt/dinit/bin"; cp "$W/dinit-root/usr/bin/dinit" "$W/dinit-root/usr/bin/dinitctl" "$W/dinit-root/usr/bin/dinit-check" "$R/opt/dinit/bin/"
cp "$HERE/../../../image/machines/hub/rootfs/usr/lib/hubos/follow-driftwm" "$R/bin/follow-driftwm" 2>/dev/null || true
# the graph is checked at build time with dinit-check (host run)
"$W/dinit-root/usr/bin/dinit-check" -d "$HERE/candidates/dinit-par/rootfs/etc/dinit.d" boot || echo "dinit-check reported a problem"
