#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# pywayland (pure test client library) into the work folder; no system install
ulimit -c 0
W=$LFT
cd $W && pip download pywayland==0.4.18 --no-deps -d pipdl 2>&1 | tail -1
pip install --no-deps --target pylib pipdl/pywayland-0.4.18-*.whl 2>&1 | tail -1
