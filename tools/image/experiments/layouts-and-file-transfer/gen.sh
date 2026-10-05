#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# generate the Python bindings for the two wlroots protocols the test clients need (pywayland ships neither)
W=$LFT
. $W/env.sh
cd $W
for x in wlr-virtual-pointer-unstable-v1.xml wlr-layer-shell-unstable-v1.xml; do
  PYTHONPATH=pylib python3 -m pywayland.scanner -i $x -o pylib/pywayland/protocol 2>&1 | tail -1
done
ls pylib/pywayland/protocol | grep -i wlr
