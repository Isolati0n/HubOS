#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
ulimit -c 0
W=$LFT
A=$W/a
O="-o Dir::State::lists=$A/apt/lists -o Dir::Cache=$A/apt/cache -o APT::Sandbox::User=root"
mkdir -p $A/ud && cd $A/ud && apt-get $O download libudev-dev 2>&1 | tail -1
dpkg -x libudev-dev*.deb $A/root
. $W/env.sh
find $A/root -name 'libudev.pc' | while read p; do sed -i "s#^prefix=/usr#prefix=$A/root/usr#; s#^libdir=/usr#libdir=$A/root/usr#; s#^includedir=/usr#includedir=$A/root/usr#" $p; done
ln -sf /usr/lib/x86_64-linux-gnu/libudev.so.1 $L/libudev.so
pkg-config --modversion libudev
