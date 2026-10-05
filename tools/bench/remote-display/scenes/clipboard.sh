SCENE_DESC="clipboard text, both ways, plain ASCII and non-ASCII (cafe with an accent, a check mark, Japanese). The hub has a virtual keyboard so the viewer window has keyboard focus."
SCENE_DUR=0
SCENE_HUB_INPUT=1
. "$(dirname "${BASH_SOURCE[0]}")/idle.sh"
repr() { python3 -c "import sys; print(repr(sys.stdin.buffer.read().decode('utf-8','backslashreplace')))"; }
scene_run() {
  sleep 3
  local ascii="hubos-ascii-test-1" utf8="café ✓ 日本 naïve" got
  # node to hub
  for pair in "ascii:$ascii" "utf8:$utf8"; do
    local name=${pair%%:*} text=${pair#*:}
    printf '%s' "$text" | node_env wl-copy 2>/dev/null; sleep 2
    got=$(hub_env timeout 5 wl-paste -n 2>/dev/null | repr)
    res "clip_node_to_hub_$name" "$got"
    [ "$got" = "$(printf '%s' "$text" | repr)" ] && res "clip_node_to_hub_${name}_exact" yes || res "clip_node_to_hub_${name}_exact" no
  done
  # hub to node
  for pair in "ascii:$ascii-2" "utf8:$utf8 two"; do
    local name=${pair%%:*} text=${pair#*:}
    printf '%s' "$text" | hub_env wl-copy 2>/dev/null; sleep 2
    got=$(node_env timeout 5 wl-paste -n 2>/dev/null | repr)
    res "clip_hub_to_node_$name" "$got"
    [ "$got" = "$(printf '%s' "$text" | repr)" ] && res "clip_hub_to_node_${name}_exact" yes || res "clip_hub_to_node_${name}_exact" no
  done
}
