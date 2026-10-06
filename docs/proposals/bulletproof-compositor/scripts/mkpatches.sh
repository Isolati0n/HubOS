#!/bin/bash
# builds the patch files for the repository from the working trees (run at the end)
B=${BC_WORK:?set BC_WORK to your work folder}
R=${REPO:?set REPO to the repository}/docs/proposals/bulletproof-compositor/patches
mkdir -p $R
d() { # name OLDTREE NEWTREE file...   (unified diff with a/ b/ prefixes, only the listed files)
  name=$1; old=$2; new=$3; shift 3
  : > $R/$name
  for f in "$@"; do
    diff -u --label a/$f --label b/$f $old/$f $new/$f >> $R/$name
  done
  echo "$name: $(grep -c '^[+-][^+-]' $R/$name) changed lines"
}
d smithay-p7-popup-parent-cycle.patch $B/sm-p6 $B/sm-p7 src/wayland/shell/xdg/handlers/surface.rs src/desktop/wayland/popup/manager.rs
d smithay-p8-subsurface-self-reorder.patch $B/sm-p7 $B/sm-p8 src/wayland/compositor/tree.rs
d smithay-p9-absurd-geometry.patch $B/sm-p8 $B/sm-p9 src/wayland/compositor/handlers.rs src/wayland/viewporter/mod.rs
d smithay-p10-positioner-clamp.patch $B/sm-p9 $B/sm src/wayland/shell/xdg/handlers/positioner.rs
d smithay-p11-negative-sizes.patch $B/sm-p9 $B/sm src/wayland/compositor/handlers.rs src/wayland/shell/xdg/handlers/surface/toplevel.rs src/wayland/shell/wlr_layer/handlers.rs
d driftwm-d1-session-entry-cap.patch $B/dwp-orig $B/dwp src/session.rs
d driftwm-d2-config-nul.patch $B/dwp-orig $B/dwp src/config/mod.rs src/config/parse.rs
d driftwm-d3-region-overflow.patch $B/dwp-orig $B/dwp src/handlers/compositor.rs
d driftwm-d4-decoration-buffer-size.patch $B/dwp-orig $B/dwp src/decorations.rs
d driftwm-d5-saturating-geometry.patch $B/dwp-orig $B/dwp src/canvas.rs src/region.rs
