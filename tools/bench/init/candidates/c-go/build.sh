# architecture C with the Go brain: the s6rc candidate plus one extra s6 service, the brain, started when the graph is up.
. "$HERE/candidates/s6rc/build.sh"
cp -a "$HERE/candidates/s6rc/rootfs/." "$R/"
cp "$W/brain-go" "$R/bin/brain"
