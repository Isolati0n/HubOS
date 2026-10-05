#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Conformance test: the reference adapter must pass; every deliberately broken variant must fail the check meant to catch it.
ulimit -c 0
W=$LFT
echo "== good adapter"; $W/conformbin $W/adapterbin | tail -2
for f in relpath dotdot missing bytes nonutf8 noise nullitems slow symleak sideeffect dirnotdir nodest-error rect exitcode; do
  res=$(BAD=$f $W/conformbin $W/adapterbin 2>&1); rc=$?
  failed=$(echo "$res" | grep '^FAIL' | awk '{print $2}' | tr '\n' ' ')
  echo "== BAD=$f: exit $rc, failed checks: ${failed:-NONE (not caught!)}"
done
