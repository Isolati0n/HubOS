import os
import re, sys, collections
rows = []
for l in open(os.environ['HS_WORK'] + '/hs/perfile.txt'):
    m = re.match(r'(\S+)\s+(\d+)\s+uw\s*(\d+) ex\s*(\d+) us\s*(\d+) un\s*(\d+)', l)
    if m: rows.append((m.group(1), *map(int, m.groups()[1:])))
G = collections.OrderedDict()
def g(name, files):
    G[name] = files
g('E1 blur (effect, UNNEEDED)', ['src/render/blur.rs', 'src/handlers/background_effect.rs'])
g('E2 shader/image backgrounds + chunked/tiled wallpapers (effect, UNNEEDED)', ['src/render/background.rs', 'src/render/capture_background.rs', 'src/render/shader_chunks.rs', 'src/render/tile_chunks.rs', 'src/render/tile_chunks_tiff.rs', 'src/render/tile_worker.rs'])
g('E3 animations (effect, UNNEEDED)', ['src/state/window_animation.rs', 'src/state/window_animation_driver.rs', 'src/render/closing.rs'])
g('E4 chrome shaders: shadow, border, corner clip (not named by owner; OPTIONAL)', ['src/render/shaders.rs', 'src/decorations.rs', 'src/render/chrome.rs'])
g('T touch + gestures + touch grabs (owner list: "no touch"; in defaults -> QUESTION)', ['src/input/touch.rs', 'src/input/gestures.rs', 'src/input/gestures/device_config.rs', 'src/input/gestures/hold.rs', 'src/input/gestures/pinch.rs', 'src/input/gestures/swipe.rs', 'src/grabs/touch_gesture_grab.rs', 'src/grabs/touch_recognizer.rs', 'src/input/tablet.rs'])
g('P pointer constraints (owner list: "no pointer locking"; in defaults -> QUESTION)', ['src/input/constraint.rs'])
g('L session lock + idle (owner list: none; in defaults -> QUESTION)', ['src/state/session_lock.rs'])
g('C screen capture: screencopy, ext-image-copy-capture, screenshots', ['src/render/capture.rs', 'src/protocols/screencopy.rs', 'src/protocols/image_copy_capture.rs', 'src/protocols/image_capture_source.rs', 'src/render/screenshot.rs'])
g('O output power, gamma, output management, workspace, foreign-toplevel, virtual keyboard', ['src/protocols/output_power.rs', 'src/protocols/gamma_control.rs', 'src/backend/gamma.rs', 'src/protocols/output_management.rs', 'src/protocols/ext_workspace.rs', 'src/protocols/foreign_toplevel.rs', 'src/protocols/virtual_keyboard.rs'])
g('X xwayland launcher (xwayland-satellite)', ['src/xwayland.rs'])
used = set(f for fs in G.values() for f in fs)
tot = [0] * 5
print('%-95s %6s %3s %3s %3s %3s' % ('group', 'lines', 'uw', 'ex', 'us', 'un'))
for name, files in G.items():
    s = [0] * 5
    for r in rows:
        if r[0] in files:
            for i in range(5): s[i] += r[i + 1]
    print('%-95s %6d %3d %3d %3d %3d   (%d files)' % (name, *s, len(files)))
    for i in range(5): tot[i] += s[i]
rest = [0] * 5
for r in rows:
    if r[0] not in used:
        for i in range(5): rest[i] += r[i + 1]
print('%-95s %6d %3d %3d %3d %3d' % ('core: everything else', *rest))
print('%-95s %6d %3d %3d %3d %3d' % ('listed groups total', *tot))
