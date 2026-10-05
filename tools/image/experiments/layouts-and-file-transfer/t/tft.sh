#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# Test: one-time signed URL transfer prototype (ft) over loopback.
ulimit -c 0
W=$LFT
X=$W/xfer; rm -rf $X; mkdir -p $X/share/proj/sub $X/share/proj/empty $X/recv $X/keys
FT=$W/ftbin
$FT genkey $X/keys/priv $X/keys/pub
echo "hello report" > $X/share/report.txt
echo dash > "$X/share/-leading dash.txt"
echo uni > "$X/share/my file (1) ünï 日本.txt"
printf 'nl\n' > "$X/share/new
line.txt"
head -c 1073741824 /dev/zero > $X/share/big.bin
echo '#!/bin/sh' > $X/share/proj/run.sh; chmod 0755 $X/share/proj/run.sh
echo data > $X/share/proj/sub/a.txt
ln -s sub/a.txt $X/share/proj/inner-link
ln -s /etc/hostname $X/share/proj/outer-link
touch -d '2020-01-02 03:04:05' $X/share/report.txt
$FT serve $X/keys/pub nas $X/share 127.0.0.1:18081 2> $X/server.log &
SP=$!; sleep 1
URL=http://127.0.0.1:18081/t
tok() { $FT sign $X/keys/priv "$1" "$2" "$3" "$4"; }
code() { curl -s -o "$5" -w "%{http_code}" "$URL/$1" ; }
echo "T1 valid token, small file:"; T=$(tok nas ai-1 $X/share/report.txt 60)
curl -s -D $X/h1 -o $X/recv/report.txt "$URL/$T"; head -1 $X/h1; cmp $X/share/report.txt $X/recv/report.txt && echo "  content identical"
grep -i last-modified $X/h1
echo "T2 same token again:"; curl -s -o /dev/null -w "  HTTP %{http_code}\n" "$URL/$T"
echo "T3 expired token (ttl -5):"; T=$(tok nas ai-1 $X/share/report.txt -5); curl -s -o /dev/null -w "  HTTP %{http_code}\n" "$URL/$T"
echo "T4 tampered path (flip one char of the payload):"; T=$(tok nas ai-1 $X/share/report.txt 60); T2="A${T:1}"; curl -s -o /dev/null -w "  HTTP %{http_code}\n" "$URL/$T2"
echo "T5 token for another sender:"; T=$(tok other ai-1 $X/share/report.txt 60); curl -s -o /dev/null -w "  HTTP %{http_code}\n" "$URL/$T"
echo "T6 path outside the shared root (/etc/hostname), correctly signed:"; T=$(tok nas ai-1 /etc/hostname 60); curl -s -o /dev/null -w "  HTTP %{http_code}\n" "$URL/$T"
echo "T6b symlink inside root pointing outside (proj/outer-link) as a single file:"; T=$(tok nas ai-1 $X/share/proj/outer-link 60); curl -s -o /dev/null -w "  HTTP %{http_code}\n" "$URL/$T"
echo "T7 awkward names:"
for n in "-leading dash.txt" "my file (1) ünï 日本.txt" "new
line.txt"; do T=$(tok nas ai-1 "$X/share/$n" 60); curl -s -o "$X/recv/got" "$URL/$T"; cmp -s "$X/share/$n" $X/recv/got && echo "  ok: $(printf %q "$n")" || echo "  FAIL: $(printf %q "$n")"; done
echo "T8 folder (tar stream) with symlinks, empty dir, mode 0755:"
T=$(tok nas ai-1 $X/share/proj 60); mkdir $X/recv/dir; curl -s "$URL/$T" | tar -x -C $X/recv/dir 2>&1
(cd $X/share && find proj | sort) > $X/l1; (cd $X/recv/dir && find proj | sort) > $X/l2; diff $X/l1 $X/l2 && echo "  same file list"
ls -l $X/recv/dir/proj/run.sh | cut -c1-10; readlink $X/recv/dir/proj/inner-link $X/recv/dir/proj/outer-link
echo "T9 1 GiB file: time, rate, server peak memory"
T=$(tok nas ai-1 $X/share/big.bin 600)
s0=$(date +%s.%N); curl -s -o $X/recv/big.bin "$URL/$T"; s1=$(date +%s.%N); echo "  1 GiB in $(echo "$s1 - $s0" | bc) s on loopback"
cmp $X/share/big.bin $X/recv/big.bin && echo "  1 GiB identical"
grep -E "VmHWM" /proc/$SP/status | sed 's/^/  server /'
echo "T10 interrupted download then resume with a Range request:"
T=$(tok nas ai-1 $X/share/big.bin 600); curl -s -o $X/recv/part.bin --max-time 0.2 "$URL/$T"; ls -l $X/recv/part.bin | awk '{print "  got " $5 " bytes before the cut"}'
curl -s -o /dev/null -w "  reusing the SAME token with Range: HTTP %{http_code}\n" -r $(stat -c %s $X/recv/part.bin)- "$URL/$T"
T=$(tok nas ai-1 $X/share/big.bin 600); curl -s -r $(stat -c %s $X/recv/part.bin)- "$URL/$T" >> $X/recv/part.bin
cmp $X/share/big.bin $X/recv/part.bin && echo "  resumed with a NEW token: identical"
echo "--- server log"; cat $X/server.log
kill $SP; rm -rf $X
