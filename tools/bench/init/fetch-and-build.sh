#!/bin/bash
# fetch-and-build.sh : get and build everything the init benchmark needs into $W (BENCH CODE). Nothing is installed:
# packages are fetched with apt-get download / curl and unpacked with dpkg -x or built from source into $W.
# These are the commands that were actually run, step by step, while the benchmark was prepared; they were NOT re-run as
# one script afterwards (UNKNOWN whether it runs unchanged). Needs: gcc, g++, make, ninja, python3, pip, go 1.24.
#   W=/some/empty/dir bash tools/bench/init/fetch-and-build.sh
set -e
: "${W:?set W to an empty work directory outside the repository}"
ulimit -c 0
mkdir -p "$W"/{apt/lists/partial,apt/cache/archives/partial,debs,root,src}
T=$W/root
APT() { apt-get -o Dir::State::lists="$W/apt/lists" -o Dir::Cache="$W/apt/cache" -o APT::Sandbox::User=root "$@"; }
CACHE() { apt-cache -o Dir::State::lists="$W/apt/lists" -o Dir::Cache="$W/apt/cache" "$@"; }
APT update >/dev/null
# 1. QEMU 8.2.2, busybox, cpio, zstd, the Ubuntu 24.04 generic kernel and its modules, runit 2.1.2, Erlang/OTP 25 and Elixir 1.14
CACHE depends --recurse --no-recommends --no-suggests --no-conflicts --no-breaks --no-replaces --no-enhances elixir | grep -E '^erlang|^elixir' | sort -u > "$W/erl-pkgs.txt"
APT install --print-uris -y --no-install-recommends qemu-system-x86 seabios qemu-system-data busybox-static cpio zstd linux-image-virtual runit \
  $(cat "$W/erl-pkgs.txt") 2>/dev/null | grep -oE "^'[^']+'" | tr -d "'" > "$W/uris.txt"
( cd "$W/debs" && while read -r u; do [ -f "$(basename "$u")" ] || curl -sS -O "$u"; done < "$W/uris.txt" )
( cd "$W/debs" && APT download linux-modules-6.8.0-146-generic linux-modules-extra-6.8.0-146-generic libcap-dev libcap2 )
for d in "$W"/debs/*.deb; do dpkg -x "$d" "$T"; done
mkdir -p "$W/modroot" "$W/capdev"
dpkg -x "$W"/debs/linux-modules-6.8.0-146-generic_*.deb "$W/modroot"
dpkg -x "$W"/debs/linux-modules-extra-6.8.0-146-generic_*.deb "$W/modroot"   # i6300esb.ko.zst lives here
for d in "$W"/debs/libcap-dev_*.deb "$W"/debs/libcap2_*.deb; do dpkg -x "$d" "$W/capdev"; done
# 2. source tarballs: skarnet.org (skalibs, execline, s6, s6-rc), Alpine's source mirror (dinit), Debian (OpenRC)
cd "$W/src"
for f in skalibs/skalibs-2.15.1.0 execline/execline-2.9.9.2 s6/s6-2.15.1.0 s6-rc/s6-rc-0.7.0.0; do curl -sS -O "https://skarnet.org/software/$f.tar.gz"; done
curl -sS -O https://distfiles.alpinelinux.org/distfiles/edge/dinit-0.21.0.tar.gz
curl -sS -O http://deb.debian.org/debian/pool/main/o/openrc/openrc_0.63.3.orig.tar.xz
for f in *.tar.*; do tar xf "$f"; done
# 3. build the skarnet suite statically into $W/skel
P=$W/skel; mkdir -p "$P"
for pk in skalibs-2.15.1.0 execline-2.9.9.2 s6-2.15.1.0 s6-rc-0.7.0.0; do
  ( cd "$W/src/$pk"
    case $pk in
      skalibs*) ./configure --prefix="$P" --enable-static-libc --disable-shared >/dev/null ;;
      *) ./configure --prefix="$P" --enable-static-libc --disable-shared --with-include="$P/include" --with-lib="$P/lib" --with-lib="$P/lib/skalibs" --with-lib="$P/lib/execline" --with-lib="$P/lib/s6" --with-dynlib="$P/lib" >/dev/null ;;
    esac
    make -j2 >/dev/null 2>&1; make install >/dev/null 2>&1 )
done
# 4. dinit (static) into $W/dinit-root
( cd "$W/src/dinit-0.21.0" && ./configure --prefix=/usr --sbindir=/sbin --disable-capabilities >/dev/null 2>&1 && echo "LDFLAGS += -static" >> mconfig \
  && make -j2 >"$W/dinit-build.log" 2>&1 && mkdir -p "$W/dinit-root" && make DESTDIR="$W/dinit-root" install >/dev/null 2>&1 )
# 5. OpenRC with meson (pip --target, no install) into $W/openrc-root
pip install --quiet --target "$W/pyt" meson
( export PYTHONPATH=$W/pyt PKG_CONFIG_PATH=$W/capdev/usr/lib/x86_64-linux-gnu/pkgconfig PKG_CONFIG_SYSROOT_DIR=$W/capdev \
         CFLAGS="-I$W/capdev/usr/include" LDFLAGS="-L$W/capdev/usr/lib/x86_64-linux-gnu -L$W/capdev/lib/x86_64-linux-gnu"
  cd "$W/src/openrc-0.63.3"
  python3 "$W/pyt/bin/meson" setup "$W/openrc-build" --prefix=/usr -Dpam=false -Dselinux=disabled -Daudit=disabled -Dbash-completions=false \
    -Dzsh-completions=false -Dnewnet=false -Dsysvinit=false -Dpkgconfig=false >/dev/null
  ninja -C "$W/openrc-build" -j2 >/dev/null
  DESTDIR="$W/openrc-root" python3 "$W/pyt/bin/meson" install -C "$W/openrc-build" >/dev/null )
# 6. the Elixir brain (host-side compile with a wrapper that points the Ubuntu erl script at the unpacked copy)
mkdir -p "$W/hostbin" "$W/ebin"
cat > "$W/hostbin/erl" <<EOF
#!/bin/sh
E=$T/usr/lib/erlang
export ROOTDIR=\$E BINDIR=\$E/erts-13.2.2.5/bin EMU=beam PROGNAME=erl
exec \$E/erts-13.2.2.5/bin/erlexec "\$@"
EOF
chmod +x "$W/hostbin/erl"
( export PATH=$W/hostbin:$T/usr/lib/elixir/bin:$PATH LD_LIBRARY_PATH=$T/usr/lib/x86_64-linux-gnu:$T/lib/x86_64-linux-gnu
  elixirc -o "$W/ebin" "$(dirname "$0")/prototypes/elixir/brain.ex" )
echo "tools ready in $W"
