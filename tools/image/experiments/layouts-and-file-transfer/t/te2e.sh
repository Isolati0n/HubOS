#!/bin/bash
: "${LFT:?set LFT to the work folder (see README.md)}"
# End-to-end sketch on loopback with two fake nodes: "copy in the GUI of nas, paste in the GUI of ai-1".
#   hub part   : asks for the selection, asks for the destination, signs one token per item, tells ai-1 where to fetch. Never reads file bytes.
#   nas part   : the adapter (selection) and the one-time-URL server (stands in for the node helper).
#   ai-1 part  : the adapter (destination, refresh) and the receiver (curl + safe naming + atomic rename; stands in for its helper).
ulimit -c 0
W=$LFT
E=$W/e2e; rm -rf $E; mkdir -p $E/nas/selection/proj/sub $E/nas/destination $E/ai/selection $E/ai/destination $E/keys
FT=$W/ftbin; AD=$W/adapterbin
$FT genkey $E/keys/priv $E/keys/pub
echo "report v1" > "$E/nas/selection/report (final).txt"
echo "nl" > "$E/nas/selection/new
line.txt"
head -c 20000000 /dev/urandom > $E/nas/selection/data.bin
echo hi > $E/nas/selection/proj/sub/a.txt; echo '#!/bin/sh' > $E/nas/selection/proj/run.sh; chmod 755 $E/nas/selection/proj/run.sh
ln -s /etc/hostname $E/nas/selection/link-out
$FT serve $E/keys/pub nas $E/nas 127.0.0.1:18101 2> $E/serve.log &
SP=$!; sleep 1

paste_once() {
  echo "-- hub: ask nas for the selection"
  SEL=$(HUBOS_FILES_FIXTURE=$E/nas $AD selection)
  echo "$SEL" | jq -c '.items[] | {name,kind,bytes}'
  echo "-- hub: ask ai-1 for the destination"
  DEST=$(HUBOS_FILES_FIXTURE=$E/ai $AD destination | jq -r .dir); echo "   dir = $DEST"
  echo "-- hub: sign one token per item (no file is opened here)"
  echo "$SEL" | jq -c '.items[]' > $E/items.jsonl
  : > $E/manifest.tsv
  while IFS= read -r line; do
    p=$(echo "$line" | jq -r .path); n=$(echo "$line" | jq -r .name); k=$(echo "$line" | jq -r .kind)
    t=$($FT sign $E/keys/priv nas ai-1 "$p" 60)
    printf '%s\t%s\t%s\n' "$k" "$t" "$(printf %s "$n" | base64 -w0)" >> $E/manifest.tsv
  done < $E/items.jsonl
  echo "-- ai-1 helper: fetch each token straight from nas, write safely into the destination"
  while IFS=$'\t' read -r k t nb; do
    n=$(printf %s "$nb" | base64 -d)
    case "$n" in */*|""|.|..) echo "   refused unsafe name"; continue;; esac
    final="$DEST/$n"; i=2
    while [ -e "$final" ]; do final="$DEST/$n ($i)"; i=$((i+1)); done
    tmp="$DEST/.hubos-part-$$-$RANDOM"
    if [ "$k" = dir ]; then
      mkdir "$tmp" && curl -sf "http://127.0.0.1:18101/t/$t" | tar -x --no-same-owner --no-same-permissions -C "$tmp" && mv "$tmp/$n" "$final" ; rmdir "$tmp" 2>/dev/null
    else
      curl -sf -o "$tmp" "http://127.0.0.1:18101/t/$t" && mv "$tmp" "$final"
    fi
    echo "   wrote: $(basename "$final")"
  done < $E/manifest.tsv
  echo '{"paths":[]}' | HUBOS_FILES_FIXTURE=$E/ai $AD refresh
}
echo "######## paste 1"; paste_once
echo "######## compare"
(cd $E/nas/selection && for f in "report (final).txt" "new
line.txt" data.bin proj/sub/a.txt proj/run.sh; do cmp -s "$f" "$E/ai/destination/$f" && echo "same: $(printf %q "$f")" || echo "DIFFERENT: $(printf %q "$f")"; done)
ls -l $E/ai/destination/proj/run.sh | cut -c1-10
echo "######## paste 2 (same files again: names must not overwrite)"; paste_once | grep wrote
echo "######## the destination now:"; ls -1 $E/ai/destination
echo "######## replaying an old token:"; t=$(awk -F'\t' 'NR==1{print $2}' $E/manifest.tsv); curl -s -o /dev/null -w "HTTP %{http_code}\n" "http://127.0.0.1:18101/t/$t"
kill $SP; rm -rf $E
