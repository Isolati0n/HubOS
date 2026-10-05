#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# unpack the test deps into $A (nothing is installed on the machine)
ulimit -c 0
W=$LFT
A=$W/a
mkdir -p $A/apt/lists/partial $A/apt/cache/archives/partial $A/debs $A/root
O="-o Dir::State::lists=$A/apt/lists -o Dir::Cache=$A/apt/cache -o APT::Sandbox::User=root"
apt-get $O update 2>&1 | tail -1
apt-get $O install --print-uris -y --no-install-recommends \
  libwayland-dev libxkbcommon-dev libinput-dev libseat-dev libgbm-dev libegl-dev \
  libgles-dev libdrm-dev libdisplay-info-dev libegl1 libgles2 libgbm1 libgl1-mesa-dri \
  libwayland-egl1 libegl-mesa0 libglx-mesa0 libxcb1-dev \
  libxkbcommon-x11-0 libxcb-xkb1 foot libfcft4t64 libutf8proc3 wayland-utils grim \
  libgudev-1.0-0 wl-clipboard sway libudev-dev \
  > $A/uris.raw 2>$A/uris.err
grep -oE "^'[^']+'" $A/uris.raw | tr -d "'" | grep -vE '/(systemd|libpam-systemd|systemd-dev|systemd-sysv|libsystemd|gir1\.2|libudev1_)' > $A/uris.txt
wc -l < $A/uris.txt
cd $A/debs
while read u; do curl -sS -O "$u"; done < $A/uris.txt
for d in $A/debs/*.deb; do dpkg -x $d $A/root; done
cat > $W/env.sh <<EOF
ulimit -c 0
W=$W
A=\$W/a
L=\$A/root/usr/lib/x86_64-linux-gnu
export PKG_CONFIG_PATH=\$L/pkgconfig:\$A/root/usr/share/pkgconfig
export LD_LIBRARY_PATH=\$L CARGO_TARGET_DIR=\$W/target CARGO_NET_GIT_FETCH_WITH_CLI=true C_INCLUDE_PATH=\$A/root/usr/include
export PATH=\$A/root/usr/bin:\$PATH
export __EGL_VENDOR_LIBRARY_DIRS=\$A/root/usr/share/glvnd/egl_vendor.d LIBGL_DRIVERS_PATH=\$L/dri GBM_BACKENDS_PATH=\$L/gbm
export LIBGL_ALWAYS_SOFTWARE=1 MESA_LOADER_DRIVER_OVERRIDE=swrast
EOF
. $W/env.sh
find $A/root -name '*.pc' | while read p; do sed -i "s#^prefix=/usr#prefix=$A/root/usr#; s#^libdir=/usr#libdir=$A/root/usr#; s#^includedir=/usr#includedir=$A/root/usr#" $p; done
for f in $(find $L -maxdepth 1 -xtype l); do b=$(basename $(readlink $f)); [ -e /usr/lib/x86_64-linux-gnu/$b ] && ln -sf /usr/lib/x86_64-linux-gnu/$b $f; done
ln -sf /usr/lib/x86_64-linux-gnu/libudev.so.1 $L/libudev.so
ldd $A/root/usr/bin/sway | grep "not found"
ldd $W/target/debug/driftwm | grep "not found"
ls $A/root/usr/bin | tr '\n' ' '
