ulimit -c 0
S=${HS_WORK}
R=$S/hubstab/rv/root
L=$S/A/root/usr/lib/x86_64-linux-gnu
rm -rf /tmp/hsq; mkdir -p /tmp/hsq; chmod 700 /tmp/hsq
export XDG_RUNTIME_DIR=/tmp/hsq DISPLAY=:81 LD_LIBRARY_PATH=$L:$R/usr/lib/x86_64-linux-gnu LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
export __EGL_VENDOR_LIBRARY_DIRS=$S/A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm
$S/hubstab/bin/driftwm-pristine --backend winit --config /dev/null > /tmp/hsq/dw.log 2>&1 &
P=$!
sleep 4
export WAYLAND_DISPLAY=wayland-1
unset DISPLAY
GDK_BACKEND=wayland $R/usr/bin/remote-viewer --name=hubos-rv -- spice://127.0.0.1:1 > /tmp/hsq/rv.log 2>&1 &
RV=$!
$S/A/root/usr/bin/foot --app-id=hubos-f sleep infinity > /tmp/hsq/foot.log 2>&1 &
F=$!
$S/A/root/usr/bin/wayland-info > /dev/null 2>&1
sleep 4
echo "before: driftwm=$(kill -0 $P && echo up) remote-viewer=$(kill -0 $RV && echo up) foot=$(kill -0 $F && echo up)"
T0=$(date +%s.%N)
kill -9 $P
for i in $(seq 1 100); do
  RVUP=$(kill -0 $RV 2>/dev/null && echo up || echo gone)
  FUP=$(kill -0 $F 2>/dev/null && echo up || echo gone)
  if [ "$RVUP" = gone ] && [ "$FUP" = gone ]; then break; fi
  sleep 0.05
done
T1=$(date +%s.%N)
echo "after kill -9: remote-viewer=$RVUP foot=$FUP after $(echo "$T1 - $T0" | bc) s"
wait $RV; echo "remote-viewer exit status: $?"
wait $F; echo "foot exit status: $?"
tail -3 /tmp/hsq/rv.log | cut -c1-200
kill $RV $F 2>/dev/null
