ulimit -c 0
S=${HS_WORK}
L=$S/A/root/usr/lib/x86_64-linux-gnu
rm -rf /tmp/hsr; mkdir -p /tmp/hsr; chmod 700 /tmp/hsr
export XDG_RUNTIME_DIR=/tmp/hsr DISPLAY=:81 LD_LIBRARY_PATH=$L LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
export __EGL_VENDOR_LIBRARY_DIRS=$S/A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm
printf '[decorations]\nbg_color = "#aaaaaa"\n' > /tmp/hsr/c.toml
$S/hubstab/bin/driftwm-pristine --backend winit --config /tmp/hsr/c.toml > /tmp/hsr/dw.log 2>&1 &
P=$!
sleep 4
echo "alive before: $(kill -0 $P 2>/dev/null && echo yes || echo no)"
printf '[decorations]\nbg_color = "#a\xc3\xa9aaa"\n' > /tmp/hsr/c.toml.new && mv /tmp/hsr/c.toml.new /tmp/hsr/c.toml
sleep 3
echo "alive after writing the bad config: $(kill -0 $P 2>/dev/null && echo yes || echo no)"
grep -E "panicked|char boundary" /tmp/hsr/dw.log | head -3
kill $P 2>/dev/null
wait $P 2>/dev/null; echo "exit status: $?"
