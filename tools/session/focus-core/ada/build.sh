#!/bin/sh
# Builds the Ada candidate with GNAT 13 unpacked (dpkg -x, nothing installed) into a folder.
# Usage: GNAT_ROOT=/path/to/unpacked/root sh build.sh     (output in ./out)
set -e
cd "$(dirname "$0")"
R=${GNAT_ROOT:?set GNAT_ROOT to the folder the gnat-13, gnat-13-x86-64-linux-gnu and libgnat-13 packages were unpacked into}
export PATH=$R/usr/bin:$PATH
export LD_LIBRARY_PATH=$R/usr/lib/x86_64-linux-gnu
export COMPILER_PATH=$R/usr/libexec/gcc/x86_64-linux-gnu/13
mkdir -p out
for p in fuzz_run trace_run; do
  gnatmake-13 -f -O2 -gnata -gnato -gnat2012 -D out -o out/$p src/$p.adb -Isrc \
    -largs -L$R/usr/lib/x86_64-linux-gnu -L$R/usr/lib/gcc/x86_64-linux-gnu/13/adalib
done
