#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# download + unpack rsync and the sftp tools into $A/xroot (nothing installed)
ulimit -c 0
W=$LFT
A=$W/a
O="-o Dir::State::lists=$A/apt/lists -o Dir::Cache=$A/apt/cache -o APT::Sandbox::User=root"
mkdir -p $A/xdebs $A/xroot && cd $A/xdebs
apt-get $O install --print-uris -y --no-install-recommends rsync openssh-client openssh-sftp-server 2>/dev/null | grep -oE "^'[^']+'" | tr -d "'" > ../xuris.txt
cat ../xuris.txt | sed 's#.*/##' | tr '\n' ' '; echo
while read u; do curl -sS -O "$u"; done < ../xuris.txt
for d in *.deb; do dpkg -x $d $A/xroot; done
ls $A/xroot/usr/bin | tr '\n' ' '
ls $A/xroot/usr/lib/openssh
export LD_LIBRARY_PATH=$A/xroot/usr/lib/x86_64-linux-gnu:$A/xroot/lib/x86_64-linux-gnu
ldd $A/xroot/usr/bin/rsync | grep "not found"
ldd $A/xroot/usr/bin/sftp | grep "not found"
$A/xroot/usr/bin/rsync --version | head -1
