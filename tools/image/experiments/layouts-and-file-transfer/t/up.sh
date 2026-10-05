#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
. $LFT/t/dw.sh
rm -rf $XDG_RUNTIME_DIR/*
start_sway
echo "parent display: $PARENT"; head -5 $T/sway.log
