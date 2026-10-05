#!/bin/sh
# blackbox-logs.sh: PROTOTYPE of the body of GET /v1/logs (text/plain). Reads the saved folders, removes lines that look like
# secrets, and writes at most MAXTOTAL bytes (newest boot first). BusyBox sh.
# usage: blackbox-logs.sh CONFIG_MOUNTPOINT
CFG=$1; MAXTOTAL=${BB_MAXTOTAL:-65536}; D=$CFG/hubos/blackbox
# What counts as a secret-looking word is a guess (BELIEVED, not complete): see the proposal.
redact() { sed -E 's/((pass(word)?|secret|token|apikey|api_key|private[_-]?key|credential)[A-Za-z_]*[=: ]+)[^ ]+/\1[REDACTED]/Ig'; }
{
echo "# Hub OS black box logs (newest boot first); redacted; cut to $MAXTOTAL bytes"
for B in $(ls -d "$D"/[0-9][0-9][0-9][0-9] 2>/dev/null | sort -r); do
  echo "## boot folder $(basename "$B")"; cat "$B/meta.txt" 2>/dev/null
  for f in "$B"/*; do
    [ "$(basename "$f")" = meta.txt ] && continue
    echo "### $(basename "$f")"
    tr -d '\000' < "$f" | redact
  done
done
} | head -c "$MAXTOTAL"
echo
