#!/bin/bash
# does a NUL character inside a config string crash the compositor?  usage: nulcfg.sh BIN
ulimit -c 0
. ${BC_WORK:?set BC_WORK to your work folder}/env.sh
BIN=$1
mkdir -p /tmp/bc4nul
printf '[keybindings]\n"alt+\\u0000" = "close-window"\n' > /tmp/bc4nul/key.toml
printf '[input.keyboard]\nlayout = "u\\u0000s"\n' > /tmp/bc4nul/layout.toml
printf '[input.keyboard]\noptions = "a\\u0000b"\n' > /tmp/bc4nul/options.toml
for f in key layout options; do
  echo "== config with a NUL character in the $f string: --check-config"
  $BIN --check-config --config /tmp/bc4nul/$f.toml 2>&1 | sed -E 's/\x1b\[[0-9;]*m//g' | grep -E "panicked|Config OK|error|warning" -A1 | head -4 | cut -c1-200
  echo "   exit status ${PIPESTATUS[0]}"
done
