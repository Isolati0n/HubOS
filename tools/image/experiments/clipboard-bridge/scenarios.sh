#!/bin/bash
# Runs the clipboard experiments against two headless compositors started by
# compositors.sh and prints every command with its output. Needs: go, and
# NH_ROOT (unpacked .debs, see README.md). Run from the repository root.
# usage: tools/image/experiments/clipboard-bridge/scenarios.sh [A|B|C|D|E|F|G]...
NH_ROOT=${NH_ROOT:-/tmp/nh-root}
export LD_LIBRARY_PATH=$NH_ROOT/usr/lib/x86_64-linux-gnu PATH=$NH_ROOT/usr/bin:$PATH
HUB=/tmp/nh-hub NODE=/tmp/nh-node
CB=$NH_ROOT/cb
HUBX=(env XDG_RUNTIME_DIR=$HUB WAYLAND_DISPLAY=wayland-1)    # "${HUBX[@]}" cmd & gives the real pid
NODEX=(env XDG_RUNTIME_DIR=$NODE WAYLAND_DISPLAY=wayland-1)
hub()  { env XDG_RUNTIME_DIR=$HUB  WAYLAND_DISPLAY=wayland-1 "$@"; }
node() { env XDG_RUNTIME_DIR=$NODE WAYLAND_DISPLAY=wayland-1 "$@"; }
say()  { printf '\n=== %s\n' "$*"; }
run()  { printf '$ %s\n' "$*"; "$@"; echo "[exit $?]"; }
LOGS=$(mktemp -d /tmp/nh-logs.XXXXXX)
trap 'rm -rf "$LOGS" /tmp/nh-big.txt' EXIT
go build -o "$CB" ./tools/image/experiments/clipboard-bridge || exit 1
API=127.0.0.1:8480
start_api() { # args passed to node-api
	"${NODEX[@]}" nice -n 15 "$CB" node-api -listen $API "$@" > $NODE/api.log 2>&1 &
	APIPID=$!; sleep 0.5
}
stop_api() { kill $APIPID 2>/dev/null; wait $APIPID 2>/dev/null; return 0; }
clear_both() { for s in hub node; do $s wl-copy --clear; $s wl-copy --primary --clear; done; sleep 0.3; }
count_wlcopy() { ps -eo stat=,comm= | awk '$1 !~ /^Z/ && $2 == "wl-copy"' | wc -l; }
count_zombies() { ps -eo stat=,comm= | awk '$1 ~ /^Z/ && $2 == "wl-copy"' | wc -l; }

want="${*:-A B C D F G E}"
for s in $want; do case $s in

A) say "A. push, pull, and the plain cases"
	start_api
	clear_both
	hub wl-copy --type 'text/plain;charset=utf-8' 'from the hub: café ✓ 日本'
	run $CB hub-push -hub-env $HUB
	printf 'node sees: '; node wl-paste -n; echo
	say "A2. same text pushed again: the node does nothing"
	run $CB hub-push -hub-env $HUB
	say "A3. node text pulled to the hub"
	node wl-copy 'from the node, line1
line2'
	run $CB hub-pull -hub-env $HUB
	printf 'hub sees: '; hub wl-paste -n; echo
	say "A4. empty and cleared hub clipboard"
	echo -n | hub wl-copy
	run $CB hub-push -hub-env $HUB
	hub wl-copy --clear; sleep 0.2
	run $CB hub-push -hub-env $HUB
	say "A5. API direct: bad JSON, empty, invalid UTF-8, NUL, unknown field, wrong method"
	for body in 'not json' '{"text":""}' '{"text":"a\u0000b"}' '{"text":"ok-with-extra","future":1}'; do
		printf '%s -> ' "$body"; curl -s -w ' [HTTP %{http_code}]\n' -X PUT -d "$body" http://$API/v1/clipboard
	done
	printf 'invalid UTF-8 byte -> '; printf '{"text":"a\377b"}' | curl -s -w ' [HTTP %{http_code}]\n' -X PUT --data-binary @- http://$API/v1/clipboard
	printf 'DELETE -> '; curl -s -w ' [HTTP %{http_code}]\n' -X DELETE http://$API/v1/clipboard
	printf 'GET other path -> '; curl -s -w ' [HTTP %{http_code}]\n' http://$API/v1/nothing
	say "A6. rate limit (20 sets per 10 s in this program)"
	for i in $(seq 1 25); do curl -s -o /dev/null -w '%{http_code} ' -X PUT -d "{\"text\":\"r$i\"}" http://$API/v1/clipboard; done; echo
	stop_api ;;

B) say "B. size limits (limit 1048576 bytes of text)"
	start_api
	for n in 1048576 1048577; do
		head -c $n /dev/zero | tr '\0' 'a' > /tmp/nh-big.txt
		printf '%s bytes through the API -> ' $n
		( printf '{"text":"'; cat /tmp/nh-big.txt; printf '"}' ) | curl -s -o /dev/null -w '[HTTP %{http_code}] %{time_total}s\n' -X PUT --data-binary @- http://$API/v1/clipboard
	done
	printf 'node wl-paste of the 1 MiB text: '; node wl-paste -n | wc -c
	printf 'GET with a node clipboard over the limit (2 MiB): '
	head -c 2097152 /dev/zero | tr '\0' 'b' | node wl-copy --type 'text/plain;charset=utf-8'
	curl -s -w ' [HTTP %{http_code}]\n' http://$API/v1/clipboard
	printf 'worst-case JSON escaping: 174762 control characters written as \\u0001 (a body six times the text): '
	( printf '{"text":"'; for i in $(seq 1 174762); do printf '\\u0001'; done; printf '"}' ) | curl -s -o /dev/null -w '[HTTP %{http_code}] body %{size_upload} bytes\n' -X PUT --data-binary @- http://$API/v1/clipboard
	say "B2. the compositor and wl-copy alone, no bridge: how big and how long"
	for mb in 1 16 100; do
		head -c $((mb*1048576)) /dev/zero | tr '\0' 'c' > /tmp/nh-big.txt
		s=$(date +%s.%N); node wl-copy --type 'text/plain;charset=utf-8' < /tmp/nh-big.txt
		got=$(node wl-paste -n | wc -c); e=$(date +%s.%N)
		printf '%s MiB: wl-copy then wl-paste returned %s bytes in %.2f s\n' $mb $got $(echo "$e - $s" | bc)
	done
	node wl-copy --clear
	say "B3. hub-watch with a hub text over the limit"
	hub wl-copy --clear; sleep 0.2
	"${HUBX[@]}" nice -n 15 "$CB" hub-watch -hub-env $HUB -max 1000 > $LOGS/watch.log 2>&1 &
	W=$!; sleep 0.6
	head -c 1001 /dev/zero | tr '\0' 'd' | hub wl-copy --type 'text/plain;charset=utf-8'; sleep 0.4
	head -c 1000 /dev/zero | tr '\0' 'e' | hub wl-copy --type 'text/plain;charset=utf-8'; sleep 0.4
	kill $W; cat $LOGS/watch.log
	stop_api ;;

C) say "C. regular selection versus primary selection"
	hub wl-copy --clear; hub wl-copy --primary --clear; sleep 0.2
	"${HUBX[@]}" nice -n 15 "$CB" hub-watch -hub-env $HUB > $LOGS/watch.log 2>&1 &
	W=$!; sleep 0.6
	hub wl-copy --primary 'only primary'; sleep 0.4
	hub wl-copy 'only regular'; sleep 0.4
	kill $W; cat $LOGS/watch.log
	printf 'regular: '; hub wl-paste -n; echo; printf 'primary: '; hub wl-paste -n --primary; echo
	say "C2. what wl-paste --watch --primary sees (a separate watch)"
	"${HUBX[@]}" wl-paste -n --primary --watch sh -c 'echo "PRIMARY-EVENT $(cat)"' > $LOGS/pw.log 2>&1 &
	W=$!; sleep 0.5; hub wl-copy --primary 'sel1'; sleep 0.3; hub wl-copy --primary 'sel2'; sleep 0.3; hub wl-copy 'reg'; sleep 0.3
	kill $W; cat $LOGS/pw.log ;;

D) say "D. echo loops: hub -> node (push) -> node clipboard -> relay (stands in for wayvnc + viewer) -> hub"
	for case in "none:-no-guard:-always-set" "hash-rule-on-hub::-always-set" "unchanged-rule-on-node:-no-guard:" "both-rules::"; do
		IFS=: read -r name hubflag nodeflag <<< "$case"
		say "D. rules: $name   (hub-watch -auto $hubflag, node-api $nodeflag)"
		clear_both
		start_api $nodeflag
		"${NODEX[@]}" nice -n 15 "$CB" relay -node-env $NODE -hub-env $HUB > $LOGS/relay.log 2>&1 & R=$!
		"${HUBX[@]}" nice -n 15 "$CB" hub-watch -hub-env $HUB -auto $hubflag -node http://$API > $LOGS/watch.log 2>&1 & W=$!
		sleep 0.8
		hub wl-copy --type 'text/plain;charset=utf-8' 'ping'
		sleep 3
		kill $W $R 2>/dev/null; stop_api
		echo "hub-watch events in 3 s: $(grep -c '^event' $LOGS/watch.log)   relay copies: $(grep -c 'relay.*->' $LOGS/relay.log)   pushes answered 429: $(grep -c rate_limited $LOGS/watch.log)"
		head -4 $LOGS/watch.log
	done
	say "D5. the viewer changes line endings (LF to CRLF) on the way; hub rule only"
	clear_both
	start_api -always-set
	"${NODEX[@]}" nice -n 15 "$CB" relay -node-env $NODE -hub-env $HUB -crlf > $LOGS/relay.log 2>&1 & R=$!
	"${HUBX[@]}" nice -n 15 "$CB" hub-watch -hub-env $HUB -auto -node http://$API > $LOGS/watch.log 2>&1 & W=$!
	sleep 0.8
	hub wl-copy --type 'text/plain;charset=utf-8' 'two
lines'
	sleep 3
	kill $W $R 2>/dev/null; stop_api
	echo "hub-watch events in 3 s: $(grep -c '^event' $LOGS/watch.log)   relay copies: $(grep -c 'relay.*->' $LOGS/relay.log)"
	cat $LOGS/watch.log
	printf 'hub clipboard now has a CR: '; hub wl-paste -n | grep -c "$(printf '\r')" ;;

E) say "E. what wl-copy leaves running, and a helper restart"
	start_api -rate 0
	clear_both
	printf 'before: wl-copy holders %s, zombies %s\n' "$(count_wlcopy)" "$(count_zombies)"
	for i in $(seq 1 100); do curl -s -o /dev/null -X PUT -d "{\"text\":\"t$i\"}" http://$API/v1/clipboard; done
	printf '100 sets done. wl-copy holders alive: %s, zombies: %s\n' "$(count_wlcopy)" "$(count_zombies)"
	ps -eo pid,ppid,stat,args | grep '[w]l-copy --type' | cut -c1-100
	say "E2. kill -9 the helper (node-api): does the node still paste?"
	kill -9 $APIPID; wait $APIPID 2>/dev/null; sleep 0.3
	printf 'node paste after helper was killed: '; node wl-paste -n; echo
	ps -eo pid,ppid,stat,args | grep '[w]l-copy --type' | cut -c1-100
	say "E3. new helper starts; sends the same text, then another"
	start_api -rate 0
	curl -s -X PUT -d '{"text":"t100"}' http://$API/v1/clipboard; echo
	curl -s -X PUT -d '{"text":"after-restart"}' http://$API/v1/clipboard; echo
	printf 'node paste: '; node wl-paste -n; echo
	printf 'wl-copy processes now: '; count_wlcopy
	say "E4. paste-once (-o): the holder exits after the first paste"
	node wl-copy -o 'once'; sleep 0.3; printf 'alive before paste: '; count_wlcopy
	node wl-paste -n; echo; sleep 0.3; printf 'alive after first paste: '; count_wlcopy
	printf 'second paste: '; node wl-paste -n; echo "[exit $?]"
	say "E5. node compositor restarts: the selection goes with it"
	node wl-copy 'before-restart'; sleep 0.2
	kill "$(cat $NODE/pid)"; sleep 1; printf 'wl-copy processes alive: '; count_wlcopy
	stop_api ;;

F) say "F. what the display-protocol side would see: types offered by wl-copy"
	node wl-copy --type 'text/plain;charset=utf-8' 'x'; sleep 0.2; run node wl-paste --list-types
	printf '<html><body>x</body></html>' | node wl-copy; sleep 0.2; echo 'without --type, html-looking text offers:'; node wl-paste --list-types | head -2
	run node wl-copy --help ;;
G) say "G. how wl-paste --watch behaves (hub side), and empty versus cleared"
	hub wl-copy --clear; hub wl-copy --primary --clear; sleep 0.2
	"${HUBX[@]}" wl-paste -n --watch sh -c 'echo "EVENT state=$CLIPBOARD_STATE bytes=$(wc -c)"' > $LOGS/g.log 2>&1 &
	W=$!; sleep 0.6
	echo "(1) started on a cleared clipboard; now 'one', 'one' again, 'two', empty text, --clear:"
	for step in "wl-copy one" "wl-copy one" "wl-copy two" "wl-copy-empty" "wl-copy --clear"; do
		case "$step" in
		wl-copy-empty) printf '' | hub wl-copy ;;
		*) hub $step ;;
		esac
		sleep 0.4
	done
	kill $W; cat $LOGS/g.log
	echo "(2) after an empty-text copy, wl-paste says:"; printf '' | hub wl-copy; sleep 0.2; hub wl-paste -n; echo "[exit $?]"
	echo "    after --clear, wl-paste says:"; hub wl-copy --clear; sleep 0.2; hub wl-paste -n; echo "[exit $?]"
	echo "(3) an image copied; wl-paste --type text:"
	printf '\211PNG\r\n\032\nxxxx' | hub wl-copy --type image/png; sleep 0.2
	hub wl-paste -n --type text; echo "[exit $?]"
	"${HUBX[@]}" wl-paste -n --type text --watch sh -c 'echo "TEXT-WATCH bytes=$(wc -c)"' > $LOGS/g2.log 2>&1 &
	W=$!; sleep 0.5
	printf '\211PNG\r\n\032\nyyyy' | hub wl-copy --type image/png; sleep 0.4
	hub wl-copy --type 'text/plain;charset=utf-8' 'after-image'; sleep 0.4
	kill $W; echo "    watch with --type text (image copy gives no event, text does):"; cat $LOGS/g2.log
	hub wl-copy --clear
	echo "(4) which types does wl-copy offer, with and without xdg-mime and file on PATH, forced type or not:"
	WB=$NH_ROOT/usr/bin
	mkdir -p /tmp/nh-empty
	env PATH=/tmp/nh-empty XDG_RUNTIME_DIR=$NODE WAYLAND_DISPLAY=wayland-1 $WB/wl-copy 'no tools'; sleep 0.2
	echo "    no tools, no --type:      $(node wl-paste --list-types | tr '\n' ' ')"
	env PATH=/tmp/nh-empty XDG_RUNTIME_DIR=$NODE WAYLAND_DISPLAY=wayland-1 $WB/wl-copy --type 'text/plain;charset=utf-8' 'no tools'; sleep 0.2
	echo "    no tools, forced type:    $(node wl-paste --list-types | tr '\n' ' ')"
	printf '<html><body>x</body></html>' | node wl-copy; sleep 0.2
	echo "    html-looking, no --type:  $(node wl-paste --list-types | tr '\n' ' ')"
	printf '#!/bin/sh\necho hi\n' | node wl-copy; sleep 0.2
	echo "    script-looking, no --type: $(node wl-paste --list-types | tr '\n' ' ')"
	printf '<html><body>x</body></html>' | node wl-copy --type 'text/plain;charset=utf-8'; sleep 0.2
	echo "    html-looking, forced type: $(node wl-paste --list-types | tr '\n' ' ')"
	node wl-copy --clear; rmdir /tmp/nh-empty ;;
esac; done
