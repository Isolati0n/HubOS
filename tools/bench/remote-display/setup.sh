#!/bin/bash
# Downloads the .deb files and source tags, unpacks/builds into $BENCH_TMP. Re-runnable. Nothing is installed.
# usage: setup.sh [debs|build|all]
set -e
. "$(dirname "$0")/env.sh"
DEBS=(sway cage weston freerdp3-wayland freerdp3-x11 tigervnc-viewer virt-viewer foot mpv grim wl-clipboard
  wtype xwayland x11-xkb-utils wayland-utils xdotool xclip xvfb xauth fonts-dejavu-core fonts-noto-core
  libfuse3-3 wlr-randr libwayland-dev libpixman-1-dev libdrm-dev libxkbcommon-dev libjansson-dev
  libgnutls28-dev libturbojpeg0-dev zlib1g-dev libwayland-bin wayland-protocols libegl-dev libgbm-dev
  libgles-dev libavcodec-dev libavutil-dev libswscale-dev libavfilter-dev libxkbcommon-dev libvncserver-dev)
# pinned source tags / commits (read 2026-10-05)
WAYVNC_TAG=v0.10.2; NEATVNC_TAG=v1.0.3; AML_TAG=v1.0.0
WLVNCC_COMMIT=cc0abf87c37920540f2439a556e6a480c28f8f46
do_debs() {
  local A=$BENCH_TMP/a; mkdir -p $A/apt/lists/partial $A/apt/cache/archives/partial $A/debs $BENCH_ROOT
  local O="-o Dir::State::lists=$A/apt/lists -o Dir::Cache=$A/apt/cache -o APT::Sandbox::User=root"
  [ -d $A/apt/lists/archive.ubuntu.com_ubuntu_dists_noble_InRelease ] || apt-get $O update >/dev/null
  apt-get $O install --print-uris -y --no-install-recommends "${DEBS[@]}" 2>/dev/null | grep -oE "^'[^']+'" | tr -d "'" > $A/uris.txt
  (cd $A/debs && xargs -P8 -n1 curl -sSfO < $A/uris.txt)
  for d in $A/debs/*.deb; do dpkg -x "$d" $BENCH_ROOT; done
  # dev packages ship libX.so -> libX.so.N links that point at libraries installed on the system, not in our folder: re-point them
  for l in $(find $BENCH_ROOT/usr/lib/x86_64-linux-gnu -maxdepth 1 -xtype l); do
    t=$(readlink $l); [ -e /usr/lib/x86_64-linux-gnu/$t ] && ln -sfn /usr/lib/x86_64-linux-gnu/$t $l
    [ -e /lib/x86_64-linux-gnu/$t ] && ln -sfn /lib/x86_64-linux-gnu/$t $l
  done
  mkdir -p $BENCH_TMP/py && pip install -q --target $BENCH_TMP/py meson 2>/dev/null || true
}
do_build() {
  mkdir -p $BENCH_SRC; cd $BENCH_SRC
  for r in wayvnc neatvnc aml wlvncc; do [ -d $r ] || git clone -q https://github.com/any1/$r.git; done
  export PKG_CONFIG_PATH=$BENCH_ROOT/usr/lib/x86_64-linux-gnu/pkgconfig:$BENCH_ROOT/usr/share/pkgconfig:$BENCH_PREFIX/lib/pkgconfig
  export CPATH=$BENCH_ROOT/usr/include:$BENCH_ROOT/usr/include/libdrm:$BENCH_ROOT/usr/include/x86_64-linux-gnu
  export LIBRARY_PATH=$BENCH_ROOT/usr/lib/x86_64-linux-gnu:$BENCH_ROOT/lib/x86_64-linux-gnu
  local M="python3 -m mesonbuild.mesonmain"
  # aml and neatvnc go in as subprojects of wayvnc (same as docs/proposals/remote-display.md round 2)
  git -C aml checkout -q $AML_TAG; git -C neatvnc checkout -q $NEATVNC_TAG; git -C wayvnc checkout -q $WAYVNC_TAG
  mkdir -p wayvnc/subprojects; ln -sfn ../../aml wayvnc/subprojects/aml; ln -sfn ../../neatvnc wayvnc/subprojects/neatvnc
  mkdir -p neatvnc/subprojects; ln -sfn ../../aml neatvnc/subprojects/aml
  if [ ! -x $BENCH_PREFIX/bin/wayvnc ]; then
    (cd wayvnc && $M setup build --prefix=$BENCH_PREFIX -Dtests=false -Dpam=disabled -Dman-pages=disabled \
      -Dscreencopy-dmabuf=disabled -Dneatvnc:h264=disabled -Dneatvnc:gbm=disabled -Dneatvnc:examples=false \
      --default-library=shared && ninja -C build -j2 && ninja -C build install)
  fi
  git -C wlvncc checkout -q $WLVNCC_COMMIT
  if [ ! -x $BENCH_PREFIX/bin/wlvncc ]; then
    mkdir -p wlvncc/subprojects; ln -sfn ../../aml wlvncc/subprojects/aml; ln -sfn ../../neatvnc wlvncc/subprojects/neatvnc
    (cd wlvncc && $M setup build --prefix=$BENCH_PREFIX && ninja -C build -j2 && ninja -C build install)
  fi
}
case ${1:-all} in debs) do_debs;; build) do_build;; all) do_debs; do_build;; esac
