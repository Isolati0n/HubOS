#!/bin/bash
ulimit -c 0
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
mkdir -p /tmp/bc4cc; printf '[session]\nrestore_windows = true\nrestore_bookmarks = true\n\n[effects]\nblur = false\n' > /tmp/bc4cc/a.toml
printf '[session]\nrestore_windows = true\nrestore_bookmarks = true\n' > /tmp/bc4cc/b.toml
for f in a b; do echo "== $f"; $W/bin/driftwm-p20 --check-config --config /tmp/bc4cc/$f.toml 2>&1 | sed -E 's/\x1b\[[0-9;]*m//g' | head -5; done
grep -n "^\[effects\]" -A12 $W/dwp/config.reference.toml | head -20
