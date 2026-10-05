ulimit -c 0
S=${HS_WORK}
export LD_LIBRARY_PATH=$S/A/root/usr/lib/x86_64-linux-gnu
printf '[decorations]\nbg_color = "#a\xc3\xa9aaa"\n' > /tmp/hs-badcolor.toml
cat /tmp/hs-badcolor.toml
cd /tmp
$S/hubstab/bin/driftwm-pristine --check-config --config /tmp/hs-badcolor.toml 2>&1 | head -8
echo "exit code: $?"
