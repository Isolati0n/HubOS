#!/bin/bash
# Prints the exact versions used (from the downloaded .deb files and the source checkouts). Output goes to results/versions.txt.
. "$(dirname "$0")/env.sh"
echo "# versions used, $(date -u +%F)"
echo "kernel: $(uname -sr)   cpu: $(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2 | sed 's/^ //')   cores: $(nproc)"
for p in sway cage weston libweston-13-0 wlroots libwlroots12 libwlroots12t64 libneatvnc0 freerdp3-wayland freerdp3-x11 libfreerdp3-3 tigervnc-viewer virt-viewer libgtk-vnc-2.0-0 foot mpv grim wl-clipboard wlr-randr xwayland x11-xkb-utils; do
  f=$(ls "$BENCH_TMP"/a/debs/${p}_*.deb 2>/dev/null | head -1); [ -n "$f" ] && echo "deb $p $(dpkg-deb -f "$f" Version)"
done
for r in wayvnc neatvnc aml wlvncc; do echo "source $r $(git -C "$BENCH_SRC/$r" describe --tags --always 2>/dev/null) $(git -C "$BENCH_SRC/$r" rev-parse HEAD) $(git -C "$BENCH_SRC/$r" log -1 --format=%cs)"; done
echo "built: $(wayvnc --version 2>&1 | tr '\n' ' ')"
echo "tools: $(go version) ; $(gcc --version | head -1) ; python $(python3 --version 2>&1) ; ffmpeg $(ffmpeg -version | head -1 | cut -d' ' -f3)"
