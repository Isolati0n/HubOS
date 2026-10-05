#!/usr/bin/env python3
"""Count panic sites in driftwm non-test code. Strips #[cfg(test)] items by brace matching,
skips src/tests/, files named tests.rs / *_tests.rs / mock.rs, strips comments and string literals roughly."""
import os, re, sys, json
ROOT = sys.argv[1]
PAT = {
 'unwrap': re.compile(r'\.unwrap\(\)'),
 'expect': re.compile(r'\.expect\('),
 'unreachable': re.compile(r'\bunreachable!\('),
 'panic': re.compile(r'\bpanic!\('),
 'unsafe': re.compile(r'\bunsafe\b'),
 'assert': re.compile(r'\b(?:assert|assert_eq|assert_ne)!\('),
 'debug_assert': re.compile(r'\bdebug_assert(?:_eq|_ne)?!\('),
 'todo': re.compile(r'\b(?:todo|unimplemented)!\('),
 'unwrap_err': re.compile(r'\.unwrap_err\(\)|\.expect_err\('),
}
def is_test_file(p):
    b = os.path.basename(p)
    return '/src/tests/' in p or b in ('tests.rs','mock.rs') or b.endswith('_tests.rs') or '/tests/' in p
def strip_cfg_test(lines):
    out = []; i = 0; n = len(lines)
    removed = set()
    while i < n:
        if re.match(r'\s*#\[cfg\(test\)\]', lines[i]):
            # skip to the item and its braces
            j = i; depth = 0; started = False
            while j < n:
                l = lines[j]
                for ch in l:
                    if ch == '{': depth += 1; started = True
                    elif ch == '}': depth -= 1
                removed.add(j)
                if started and depth <= 0: break
                if not started and l.rstrip().endswith(';') and j > i: break
                j += 1
            i = j + 1
        else:
            i += 1
    return removed
res = {}; sites = []
nfiles = 0; nlines = 0
for dp, dn, fn in os.walk(os.path.join(ROOT, 'src')):
    for f in fn:
        if not f.endswith('.rs'): continue
        p = os.path.join(dp, f)
        if is_test_file(p): continue
        lines = open(p, encoding='utf8', errors='replace').read().split('\n')
        rem = strip_cfg_test(lines)
        nfiles += 1
        rel = os.path.relpath(p, ROOT)
        for idx, l in enumerate(lines):
            if idx in rem: continue
            nlines += 1
            code = re.sub(r'//.*$', '', l)
            code = re.sub(r'"(?:[^"\\]|\\.)*"', '""', code)
            for k, rx in PAT.items():
                c = len(rx.findall(code))
                if c:
                    res.setdefault(rel, {}).setdefault(k, 0)
                    res[rel][k] += c
                    sites.append((rel, idx+1, k, l.strip()))
tot = {}
for r in res.values():
    for k, v in r.items(): tot[k] = tot.get(k, 0) + v
print('files', nfiles, 'lines(non-test, comments incl.)', nlines)
print(tot)
json.dump(sites, open(sys.argv[2], 'w'), indent=0)
top = sorted(res.items(), key=lambda kv: -(kv[1].get('unwrap',0)+kv[1].get('expect',0)))[:15]
for f, r in top: print(f, r)
