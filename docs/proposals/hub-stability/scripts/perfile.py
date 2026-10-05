import os, re, sys, json
ROOT = sys.argv[1]
def is_test_file(p):
    b = os.path.basename(p)
    return '/src/tests/' in p or b in ('tests.rs', 'mock.rs') or b.endswith('_tests.rs') or '/tests/' in p
def strip_cfg_test(lines):
    n = len(lines); removed = set(); i = 0
    while i < n:
        if re.match(r'\s*#\[cfg\(test\)\]', lines[i]):
            j = i; depth = 0; started = False
            while j < n:
                for ch in lines[j]:
                    if ch == '{': depth += 1; started = True
                    elif ch == '}': depth -= 1
                removed.add(j)
                if started and depth <= 0: break
                if not started and lines[j].rstrip().endswith(';') and j > i: break
                j += 1
            i = j + 1
        else:
            i += 1
    return removed
sites = json.load(open(sys.argv[2]))
cnt = {}
for f, l, k, t in sites:
    cnt.setdefault(f, {}).setdefault(k, 0); cnt[f][k] += 1
rows = []
for dp, dn, fn in os.walk(ROOT + '/src'):
    for f in fn:
        if not f.endswith('.rs'): continue
        p = os.path.join(dp, f)
        if is_test_file(p): continue
        lines = open(p, errors='replace').read().split('\n')
        rem = strip_cfg_test(lines)
        code = 0
        for i, l in enumerate(lines):
            if i in rem: continue
            s = l.strip()
            if s and not s.startswith('//'): code += 1
        rel = os.path.relpath(p, ROOT)
        c = cnt.get(rel, {})
        rows.append((rel, code, c.get('unwrap', 0), c.get('expect', 0), c.get('unsafe', 0), c.get('unreachable', 0) + c.get('panic', 0)))
rows.sort()
tot = [0] * 5
for r in rows:
    print('%-45s %5d  uw%3d ex%2d us%2d un%2d' % r)
    for i in range(5): tot[i] += r[i + 1]
print('TOTAL code lines (non-comment, non-blank, non-cfg(test)), unwrap, expect, unsafe, unreachable+panic:', tot)
