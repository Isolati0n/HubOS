import json, os
rows = [
 ('min: no background shader, animation_speed 1.0, no blur', '/tmp/hs-llvm-min'),
 ('min, repeated (second run)', '/tmp/hs-llvm2-min'),
 ('min with LP_NUM_THREADS=2 (llvmpipe limited to 2 threads)', '/tmp/hs-llvm2-min-lp2'),
 ('default config (built-in dot-grid shader background, default animation speed)', '/tmp/hs-llvm2-default'),
 ('shader: animated smoke shader background (fast_smoke.glsl)', '/tmp/hs-llvm2-shader'),
 ('blur: every window blurred and 90% opaque', '/tmp/hs-llvm-blur'),
 ('heavy: shader background + blur', '/tmp/hs-llvm-heavy'),
]
print('| Variant (20 foot windows, 700x525 each, nested 3840x2160) | idle: CPU % of one core | pan (camera glides): CPU ms per frame event, events/s | 20 windows moving: CPU ms per frame event, events/s | RSS MB |')
print('|---|---|---|---|---|')
for name, d in rows:
    p = d + '/result.json'
    if not os.path.exists(p):
        print('| %s | not run / failed | | | |' % name); continue
    r = json.load(open(p))
    def g(k, f):
        x = r.get(k)
        return f(x) if x else 'n/a'
    idle = g('idle', lambda x: '%.0f%%' % x['cpu_pct_of_one_core'])
    pan = g('camera_glides', lambda x: '%s ms, %.1f/s' % (x.get('cpu_ms_per_frame', 'n/a') if x.get('cpu_ms_per_frame', 0) < 5000 else '%.0f' % x['cpu_ms_per_frame'], x['fps']))
    mov = g('windows_moving', lambda x: '%s ms, %.1f/s' % (x.get('cpu_ms_per_frame', 'n/a'), x['fps']))
    rss = g('windows_moving', lambda x: x['rss_mb'])
    print('| %s | %s | %s | %s | %s |' % (name, idle, pan, mov, rss))
