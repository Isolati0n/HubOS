import os
import re, collections
rows = {}
for l in open(os.environ['HS_WORK'] + '/hs/perfile.txt'):
    m = re.match(r'(\S+)\s+(\d+)\s+uw\s*(\d+) ex\s*(\d+) us\s*(\d+) un\s*(\d+)', l)
    if m: rows[m.group(1)] = tuple(map(int, m.groups()[1:]))
F = collections.OrderedDict()
def f(name, files): F[name] = files
f('Canvas: pan, zoom, momentum, camera, navigation, edge pan', ['src/canvas.rs', 'src/state/viewport.rs', 'src/state/viewport_animation.rs', 'src/state/navigation.rs', 'src/state/edge_pan.rs'])
f('Stage, windows, focus, fit/fill/fullscreen, pinning, snapping, clusters, placement, resize, move grabs', ['src/stage/element.rs', 'src/stage/mod.rs', 'src/state/cluster_snapshot.rs', 'src/state/cursor.rs', 'src/state/fill.rs', 'src/state/fit.rs', 'src/state/focus.rs', 'src/state/fullscreen.rs', 'src/state/keyboard_focus.rs', 'src/state/membership.rs', 'src/state/output.rs', 'src/state/pinned.rs', 'src/state/placement.rs', 'src/state/recenter.rs', 'src/state/redraw.rs', 'src/state/render_cache.rs', 'src/state/resize.rs', 'src/state/stage_window.rs', 'src/state/window_frame.rs', 'src/state/window_lifecycle.rs', 'src/state/errors.rs', 'src/state/mod.rs', 'src/state/init.rs', 'src/layout/auto_placement.rs', 'src/layout/cluster.rs', 'src/layout/fill.rs', 'src/layout/mod.rs', 'src/layout/snap.rs', 'src/grabs/mod.rs', 'src/grabs/move_grab.rs', 'src/grabs/resize_grab.rs', 'src/grabs/pan_grab.rs', 'src/grabs/navigate_grab.rs', 'src/grabs/screen_space_click.rs', 'src/region.rs', 'src/surface_tree.rs', 'src/window_ext.rs', 'src/lib.rs'])
f('xdg-shell, compositor and seat handlers, window rules, decorations negotiation', ['src/handlers/compositor.rs', 'src/handlers/mod.rs', 'src/handlers/xdg_shell.rs'])
f('wlr layer-shell', ['src/handlers/layer_shell.rs', 'src/state/layers.rs', 'src/render/layers.rs'])
f('Saved layouts and stand-ins (session, suspend, relaunch)', ['src/session.rs', 'src/state/session_store.rs', 'src/state/persistence.rs', 'src/state/suspended.rs', 'src/state/activation.rs', 'src/desktop_entry.rs', 'src/render/suspended.rs'])
f('IPC', ['src/ipc/client.rs', 'src/ipc/mod.rs', 'src/ipc/protocol.rs'])
f('Config and hot reload, start-up, signals', ['src/config/cvt.rs'] + ['src/config/defaults.rs', 'src/config/mod.rs', 'src/config/parse.rs', 'src/config/parse_helpers.rs', 'src/config/toml.rs', 'src/config/types.rs', 'src/state/reload.rs', 'src/main.rs', 'src/signals.rs', 'src/text.rs'])
f('Backends: udev/DRM/libinput/libseat, winit, mode timing', ['src/backend/cvt.rs', 'src/backend/mod.rs', 'src/backend/udev.rs', 'src/backend/winit.rs'])
f('Rendering core: compose, elements, cursor, damage, error bar, lifecycle', ['src/render/mod.rs', 'src/render/elements.rs', 'src/render/cursor.rs', 'src/render/error_bar.rs', 'src/render/lifecycle.rs'])
f('Input: pointer, keyboard, bindings, actions', ['src/input/mod.rs', 'src/input/pointer.rs', 'src/input/keyboard.rs', 'src/input/actions.rs'])
f('TOUCH + gestures + tablet', ['src/input/touch.rs', 'src/input/gestures.rs', 'src/input/gestures/device_config.rs', 'src/input/gestures/hold.rs', 'src/input/gestures/pinch.rs', 'src/input/gestures/swipe.rs', 'src/grabs/touch_gesture_grab.rs', 'src/grabs/touch_recognizer.rs', 'src/input/tablet.rs'])
f('POINTER CONSTRAINTS', ['src/input/constraint.rs'])
f('SESSION LOCK / idle', ['src/state/session_lock.rs'])
f('SCREEN CAPTURE', ['src/render/capture.rs', 'src/protocols/screencopy.rs', 'src/protocols/image_copy_capture.rs', 'src/protocols/image_capture_source.rs', 'src/render/screenshot.rs'])
f('OUTPUT POWER, GAMMA, OUTPUT MANAGEMENT, WORKSPACE, FOREIGN TOPLEVEL, VIRTUAL KEYBOARD', ['src/protocols/output_power.rs', 'src/protocols/gamma_control.rs', 'src/backend/gamma.rs', 'src/protocols/output_management.rs', 'src/protocols/ext_workspace.rs', 'src/protocols/foreign_toplevel.rs', 'src/protocols/virtual_keyboard.rs', 'src/protocols/mod.rs'])
f('XWAYLAND launcher', ['src/xwayland.rs'])
f('EFFECT blur', ['src/render/blur.rs', 'src/handlers/background_effect.rs'])
f('EFFECT shader/image backgrounds', ['src/render/background.rs', 'src/render/capture_background.rs', 'src/render/shader_chunks.rs', 'src/render/tile_chunks.rs', 'src/render/tile_chunks_tiff.rs', 'src/render/tile_worker.rs'])
f('EFFECT animations', ['src/state/window_animation.rs', 'src/state/window_animation_driver.rs', 'src/render/closing.rs'])
f('Shader chrome: shadow, border, corner clip', ['src/render/shaders.rs', 'src/decorations.rs', 'src/render/chrome.rs'])
used = set(x for v in F.values() for x in v)
tot = [0] * 5
for k, files in F.items():
    s = [0] * 5
    for x in files:
        if x in rows:
            for i in range(5): s[i] += rows[x][i]
    for i in range(5): tot[i] += s[i]
    print('%-100s %6d  %3d %3d %3d %3d' % (k, *s))
print('mapped total', tot)
print('unmapped:', [x for x in rows if x not in used])
