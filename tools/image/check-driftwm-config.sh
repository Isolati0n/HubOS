#!/bin/bash
# check-driftwm-config.sh ROOT [CONFIG]: the image build's check of the hub compositor's settings file (owner decisions
# 5 and 3 of 2026-10-06; docs/proposals/driftwm-patches.md section 3.2). The settings are baked into the image, so a mistake in
# them is found HERE, when the image is built, and not on the machine. It fails (exit 1, a plain message) if:
#   1. driftwm's own `--check-config` says anything but exactly "Config OK" or exits with an error. driftwm rejects an
#      unknown field in every table (serde deny_unknown_fields: SOURCE src/config/toml.rs), and it reports a bad value as a
#      warning, "Config OK, N warning(s)"; for the baked file even a warning is a failure here (strict);
#   2. [session] does not say `restore_windows = false` itself (BC-7, BC-8: a file without the line works today only by
#      default; hubd restores the windows, driftwm's own restore stays off, and this is written down in the file).
# driftwm is run INSIDE ROOT (the finished image root, which has every library it needs), as a process that only reads the
# file. CONFIG defaults to /etc/hubos/driftwm.toml (a path inside ROOT). For the test of this script, DRIFTWM_CHECK_CMD
# replaces the chroot command (it is run with `--check-config --config FILE` added).
set -u
ROOT=${1:?usage: check-driftwm-config.sh ROOT [CONFIG]}
CFG=${2:-/etc/hubos/driftwm.toml}
FILE=$ROOT$CFG
[ -f "$FILE" ] || { echo "driftwm config check: $FILE does not exist" >&2; exit 1; }
if [ -n "${DRIFTWM_CHECK_CMD:-}" ]; then
  out=$($DRIFTWM_CHECK_CMD --check-config --config "$FILE" 2>&1); rc=$?
else
  out=$(chroot "$ROOT" /usr/local/bin/driftwm --check-config --config "$CFG" 2>&1); rc=$?
fi
if [ $rc -ne 0 ] || [ "$out" != "Config OK" ]; then
  echo "driftwm config check FAILED for $CFG (exit $rc); driftwm says:" >&2
  printf '%s\n' "$out" >&2
  exit 1
fi
# restore_windows must be written in the [session] table and be false. (A tiny TOML reading: the file is ours, one key per line.)
val=$(awk '
  /^[[:space:]]*\[/ { sec = $0; gsub(/[[:space:]]/, "", sec); next }
  sec == "[session]" && /^[[:space:]]*restore_windows[[:space:]]*=/ { v = $0; sub(/^[^=]*=[[:space:]]*/, "", v); sub(/[[:space:]]*(#.*)?$/, "", v); print v }
' "$FILE")
if [ "$val" != "false" ]; then
  echo "driftwm config check FAILED for $CFG: [session] must contain 'restore_windows = false' (found: '${val:-nothing}')" >&2
  exit 1
fi
echo "driftwm config check ok: $CFG (strict, restore_windows = false written)"
