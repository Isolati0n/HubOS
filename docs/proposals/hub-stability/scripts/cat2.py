import os
import json, re, collections
W = os.environ['HS_WORK'] + '/hubstab/dw/'
sites = json.load(open(os.environ['HS_WORK'] + '/hs/sites.json'))
# manual classes for everything that is not a seat getter or a mutex lock
MAN = {}
def put(cls, items):
    for i in items: MAN[i] = cls
put('C checked just before in the same function', ['src/canvas.rs:577','src/canvas.rs:578','src/grabs/move_grab.rs:380','src/handlers/compositor.rs:281','src/backend/udev.rs:891','src/render/error_bar.rs:70','src/render/capture.rs:313','src/input/actions.rs:319','src/render/cursor.rs:138','src/state/viewport_animation.rs:90','src/config/parse.rs:51','src/config/parse.rs:73','src/input/actions.rs:862','src/render/tile_chunks.rs:790','src/render/shader_chunks.rs:476'])
put('D Option take/get of the backend or renderer (put back at the end of the same function)', ['src/backend/udev.rs:593','src/backend/udev.rs:692','src/backend/udev.rs:1502','src/backend/winit.rs:51','src/backend/winit.rs:58','src/backend/winit.rs:214'])
put('E output.current_mode() (every output is created with a mode)', ['src/backend/udev.rs:1872','src/render/capture.rs:103','src/render/capture.rs:465','src/protocols/screencopy.rs:175','src/protocols/screencopy.rs:204'])
put('F user-data get::<T>() right after insert_if_missing, or set up by the same code', ['src/handlers/layer_shell.rs:193','src/handlers/mod.rs:1371','src/handlers/mod.rs:1409','src/handlers/compositor.rs:465','src/handlers/compositor.rs:937','src/ipc/mod.rs:771'])
put('G guarded by a feature test in the caller (blur, shader background)', ['src/render/blur.rs:1048','src/render/blur.rs:1049','src/render/blur.rs:1300','src/render/background.rs:358','src/render/background.rs:467','src/render/layers.rs:361','src/render/mod.rs:1563'])
put('H protocol code the hub does not need: believed safe, not proved', ['src/protocols/screencopy.rs:268','src/protocols/screencopy.rs:426','src/protocols/output_power.rs:55','src/protocols/output_power.rs:174','src/protocols/virtual_keyboard.rs:574','src/protocols/foreign_toplevel.rs:330'])
put('S smithay calls that Smithay itself validated first (drag start)', ['src/handlers/mod.rs:161','src/handlers/mod.rs:167'])
cat = collections.Counter(); lst = collections.defaultdict(list)
cache = {}
for f, l, k, t in sites:
    if k != 'unwrap': continue
    key = f'{f}:{l}'
    L = cache.setdefault(f, open(W + f).read().split('\n'))
    ctx = ' '.join(x.strip() for x in L[max(0, l - 4):l])
    if key in MAN: c = MAN[key]
    elif re.search(r'get_(pointer|keyboard|touch)\(\)\s*\.unwrap\(\)', ctx) or (re.search(r'get_(pointer|keyboard|touch)\(\)$', L[l - 2].strip()) and '.unwrap()' in L[l - 1]):
        c = 'A seat.get_pointer/keyboard/touch().unwrap()'
    elif '.lock()' in ctx or 'cv.wait' in t or '.xkb().lock()' in ctx:
        c = 'B Mutex lock().unwrap() (poison only)'
    else:
        c = 'X UNCLASSIFIED'
    cat[c] += 1; lst[c].append(key)
tot = 0
for c, n in sorted(cat.items()):
    print(n, c); tot += n
print('total', tot)
for k in lst['X UNCLASSIFIED']: print('  unclassified', k)
for k in lst['S smithay calls that Smithay itself validated first (drag start)']: print('  S', k)
