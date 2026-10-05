SCENE_DESC="the node changes its own screen size (full size, then half size, then full size again): bytes, CPU, and whether the picture comes back (needs a stack that can resize the node)"
SCENE_DUR=0
SCENE_NEEDS=resize
. "$(dirname "${BASH_SOURCE[0]}")/idle.sh"
scene_run() {
  sleep 5
  res resize_load_before "$(load_now)"
  local a b; a=($(meter_read))
  stack_resize $((W / 2)) $((H / 2)); sleep 4
  b=($(meter_read)); res resize_down_s2c_bytes "$(( b[0] - a[0] ))"
  res resize_viewer_alive_after_down "$(kill -0 "$CLIENT_PID" 2>/dev/null && echo 1 || echo 0)"
  a=("${b[@]}")
  stack_resize "$W" "$H"
  sleep 3; b=($(meter_read)); res resize_up_s2c_bytes "$(( b[0] - a[0] ))"
  stack_capture "$RD/node.png"; hub_screenshot "$RD/hub.png"
  res resize_after_ssim "$(ssim "$RD/node.png" "$RD/hub.png")"
  res resize_viewer_alive_end "$(kill -0 "$CLIENT_PID" 2>/dev/null && echo 1 || echo 0)"
}
