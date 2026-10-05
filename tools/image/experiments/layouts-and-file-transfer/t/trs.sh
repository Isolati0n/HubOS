#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test: rsync daemon as a per-transfer server, and sftp-server speaking SFTP locally (no sshd: not startable without /run/sshd).
ulimit -c 0
W=$LFT
A=$W/a
PASS=$(head -c 12 /dev/urandom | base64 | tr -d '/+=')   # a fresh throw-away test word on every run
WRONG=x$PASS                                                # and a wrong one
export LD_LIBRARY_PATH=$A/xroot/usr/lib/x86_64-linux-gnu:$A/xroot/lib/x86_64-linux-gnu
export PATH=$A/xroot/usr/bin:$PATH
X=$W/xfer2; rm -rf $X; mkdir -p $X/share/proj/sub $X/share/proj/empty $X/recv
echo data > $X/share/proj/sub/a.txt; echo '#!/bin/sh' > $X/share/proj/run.sh; chmod 755 $X/share/proj/run.sh
ln -s sub/a.txt $X/share/proj/inner-link; ln -s /etc/hostname $X/share/proj/outer-link
head -c 536870912 /dev/zero > $X/share/proj/big.bin
echo "tok:$PASS" > $X/secrets; chmod 600 $X/secrets
cat > $X/rsyncd.conf <<EOF
port = 18873
address = 127.0.0.1
pid file = $X/rsyncd.pid
log file = $X/rsyncd.log
use chroot = no
[xfer]
  path = $X/share/proj
  read only = yes
  auth users = tok
  secrets file = $X/secrets
  hosts allow = 127.0.0.1
  uid = root
  gid = root
EOF
rsync --daemon --config=$X/rsyncd.conf; sleep 1
echo "R1 rsync daemon, right password:"
RSYNC_PASSWORD="$PASS" rsync -a --exclude big.bin rsync://tok@127.0.0.1:18873/xfer/ $X/recv/ && (cd $X/share/proj && find . ! -name big.bin | sort) > $X/l1 && (cd $X/recv && find . | sort) > $X/l2 && diff $X/l1 $X/l2 && echo "  same file list, modes: $(ls -l $X/recv/run.sh | cut -c1-10), links: $(readlink $X/recv/inner-link) $(readlink $X/recv/outer-link)"
echo "R2 wrong password:"; RSYNC_PASSWORD="$WRONG" rsync -a rsync://tok@127.0.0.1:18873/xfer/ $X/recv2/ 2>&1 | head -2
echo "R3 -L (follow links) tries to read the link that points outside the module path:"
mkdir -p $X/recv3; RSYNC_PASSWORD="$PASS" rsync -aL --exclude big.bin rsync://tok@127.0.0.1:18873/xfer/ $X/recv3/ 2>&1 | head -3; ls -l $X/recv3 | grep -E "outer|inner" | cut -c1-90
echo "R4 same password used again (nothing makes it one-time):"; RSYNC_PASSWORD="$PASS" rsync -a --exclude big.bin rsync://tok@127.0.0.1:18873/xfer/ $X/recv4/ && echo "  second use accepted"
echo "R5 interrupt a 512 MiB copy, resume with --partial --append-verify:"
mkdir -p $X/recv5; RSYNC_PASSWORD="$PASS" timeout 0.3 rsync -a --partial rsync://tok@127.0.0.1:18873/xfer/big.bin $X/recv5/ ; ls -l $X/recv5 | awk 'NR>1{print "  partial file: " $5 " bytes (" $9 ")"}'
RSYNC_PASSWORD="$PASS" rsync -a --partial --append-verify rsync://tok@127.0.0.1:18873/xfer/big.bin $X/recv5/ && cmp $X/share/proj/big.bin $X/recv5/big.bin && echo "  resumed copy identical"
kill $(cat $X/rsyncd.pid); sleep 0.5
echo "R6 after the daemon is stopped:"; RSYNC_PASSWORD="$PASS" rsync -a rsync://tok@127.0.0.1:18873/xfer/ $X/recv6/ 2>&1 | head -1
echo "--- rsync daemon log"; tail -4 $X/rsyncd.log | cut -c1-150

echo "S1 sftp client driving a local sftp-server (-D), batch:"
SFTPS=$A/xroot/usr/lib/openssh/sftp-server
cat > $X/batch <<EOF
ls -l $X/share/proj
get $X/share/proj/sub/a.txt $X/recv/got-a.txt
get -p $X/share/proj/run.sh $X/recv/got-run.sh
put $X/share/proj/sub/a.txt $X/recv/put-a.txt
EOF
sftp -b $X/batch -D $SFTPS 2>&1 | cut -c1-120 | head -14
ls -l $X/recv/got-run.sh | cut -c1-10
echo "S2 reget after a cut:"; head -c 100000000 $X/share/proj/big.bin > $X/recv/big.bin
echo "reget $X/share/proj/big.bin $X/recv/big.bin" > $X/b2; sftp -b $X/b2 -D $SFTPS 2>&1 | tail -1 | cut -c1-120; cmp $X/share/proj/big.bin $X/recv/big.bin && echo "  reget finished identical"
rm -rf $X
