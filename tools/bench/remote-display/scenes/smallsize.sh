SCENE_DESC="the viewer is asked to become smaller (its window is taken out of fullscreen and set to half size): does the node follow? Reports the node's screen size and the hub window size before and after."
SCENE_DUR=0
. "$(dirname "${BASH_SOURCE[0]}")/idle.sh"
node_size() {   # the node compositor's own idea of its output size (xdg-output; else the current wl_output mode)
  local o; o=$(node_env wayland-info 2>/dev/null)
  local v; v=$(printf '%s\n' "$o" | sed -n 's/.*logical_width: \([0-9]*\), logical_height: \([0-9]*\).*/\1x\2/p' | head -1)
  [ -z "$v" ] && v=$(printf '%s\n' "$o" | sed -n 's/.*width: \([0-9]*\) px, height: \([0-9]*\) px.*refresh.*flags: .*current.*/\1x\2/p' | head -1)
  echo "${v:-unknown}"
}
win_size() { hub_tree | python3 -c "
import json,sys
def walk(n):
    if n.get('pid') and n.get('type') in ('con','floating_con') and (n.get('app_id') or n.get('window_properties')):
        r=n['window_rect']; print('%dx%d'%(r['width'],r['height'])); return True
    for c in n.get('nodes',[])+n.get('floating_nodes',[]):
        if walk(c): return True
walk(json.load(sys.stdin))"; }
scene_run() {
  sleep 6
  res node_size_before "$(node_size)"; res hub_window_before "$(win_size)"
  local hw=$((W / 2)) hh=$((H / 2))
  hub_env swaymsg "[pid=$(leaf_pid "$CLIENT_PID")] fullscreen disable, floating enable, resize set width $hw px height $hh px" >/dev/null 2>&1
  sleep 5
  res node_size_after "$(node_size)"; res hub_window_after "$(win_size)"
  res viewer_alive_after "$(kill -0 "$CLIENT_PID" 2>/dev/null && echo 1 || echo 0)"
  hub_screenshot "$RD/hub_small.png"; res hub_window_screenshot "$(python3 -c "
import struct,sys
d=open('$RD/hub_small.png','rb').read(32); print('%dx%d'%struct.unpack('>II',d[16:24]))")"
}
