SCENE_DESC="text fidelity: the idle page of text; the node's own screenshot is compared with a screenshot of the hub (SSIM and PSNR)"
SCENE_DUR=0
. "$(dirname "${BASH_SOURCE[0]}")/idle.sh"
scene_run() {
  sleep 8
  stack_capture "$RD/node.png"; hub_screenshot "$RD/hub.png"
  local refdir=$BENCH_TMP/refs; mkdir -p "$refdir"
  # Weston's own capture only completes if the viewer keeps asking for updates (TESTED: it never completes with wlvncc on an idle screen).
  # The page of text is drawn by foot itself, so the same page at the same size is pixel-identical on every compositor
  # (TESTED: sway's node capture against the lossless remote-viewer picture of a Weston node: SSIM 1.0, PSNR inf).
  # So a capture made on the sway stack is kept as a reference and used when the stack's own capture fails.
  if [ -s "$RD/node.png" ] && [ "$STACK" = wayvnc-sway ]; then cp "$RD/node.png" "$refdir/page-${W}x${H}.png"; fi
  if [ ! -s "$RD/node.png" ] && [ -s "$refdir/page-${W}x${H}.png" ]; then cp "$refdir/page-${W}x${H}.png" "$RD/node.png"; res fidelity_reference "sway capture of the same page (the stack's own capture did not complete)"; fi
  [ -s "$RD/node.png" ] || { res fidelity_error "no node capture"; return; }
  [ -s "$RD/hub.png" ] || { res fidelity_error "no hub capture"; return; }
  local out; out=$(ssim "$RD/node.png" "$RD/hub.png"); res fidelity_full "$out"
  # the text area only (left 60% of the screen, where the text is), to keep the white background from flattering the number
  local cw=$(( W * 6 / 10 )); out=$(ssim -crop "0,0,$cw,$H" "$RD/node.png" "$RD/hub.png"); res fidelity_textarea "$out"
  if [ -n "${KEEP_SHOTS:-}" ]; then cp "$RD/node.png" "$KEEP_SHOTS.node.png"; cp "$RD/hub.png" "$KEEP_SHOTS.hub.png"; fi
}
