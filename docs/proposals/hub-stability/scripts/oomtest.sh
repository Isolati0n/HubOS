ulimit -c 0
S=${HS_WORK}
L=$S/A/root/usr/lib/x86_64-linux-gnu
rm -rf /tmp/hso; mkdir -p /tmp/hso; chmod 700 /tmp/hso
export XDG_RUNTIME_DIR=/tmp/hso DISPLAY=:81 LD_LIBRARY_PATH=$L LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
export __EGL_VENDOR_LIBRARY_DIRS=$S/A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=$L/dri GBM_BACKENDS_PATH=$L/gbm
for lim in 2500000 1800000 1300000 900000; do
  ( ulimit -v $lim; $S/hubstab/bin/driftwm-pristine --backend winit --config /dev/null > /tmp/hso/dw-$lim.log 2>&1 & echo $! > /tmp/hso/pid; wait $!; echo "ulimit -v $lim KB: driftwm exit status $?" ) 2>&1 &
  sleep 12
  P=$(cat /tmp/hso/pid); kill $P 2>/dev/null
  wait 2>/dev/null
  grep -m2 -E "memory allocation|alloc|panicked|cannot|failed" /tmp/hso/dw-$lim.log | cut -c1-160
done
