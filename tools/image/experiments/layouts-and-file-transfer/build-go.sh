#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# build the three Go test programs (standard library only) into the work folder
ulimit -c 0
W=$LFT
cd $W/ft && gofmt -l . && go vet ./... && go build -o ../ftbin . && go build -o ../adapterbin ./adapter && go build -o ../conformbin ./conform && ls -la ../ftbin ../adapterbin ../conformbin
