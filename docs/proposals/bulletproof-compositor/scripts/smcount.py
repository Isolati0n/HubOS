#!/usr/bin/env python3
"""Counts unwrap()/expect()/unreachable!/panic!/unsafe in Smithay at the pinned rev (non-test code), by pattern. usage: smcount.py SMITHAY_SRC_DIR"""
import os, re, sys, collections
root = sys.argv[1]
pats = collections.OrderedDict([
    ('lock().unwrap()  (poisoned-lock panic)', re.compile(r'\.lock\(\)\s*\.unwrap\(\)|\.read\(\)\s*\.unwrap\(\)|\.write\(\)\s*\.unwrap\(\)|\.lock\(\)\.expect')),
    ('.data::<..>().unwrap() / user_data().get().unwrap()  (set up by the same module)', re.compile(r'\.data::<[^>]*(<[^>]*>)?[^>]*>\(\)\s*\.unwrap\(\)|user_data\(\)\s*\.get(::<[^>]*>)?\(\)\s*\.unwrap\(\)')),
    ('other .unwrap()', re.compile(r'\.unwrap\(\)')),
    ('.expect(', re.compile(r'\.expect\(')),
    ('unreachable!(', re.compile(r'unreachable!\(')),
    ('panic!(', re.compile(r'\bpanic!\(')),
    ('unsafe', re.compile(r'\bunsafe\b')),
])
tot = collections.Counter(); files = collections.defaultdict(collections.Counter)
for d, _, fs in os.walk(root):
    for f in fs:
        if not f.endswith('.rs'): continue
        p = os.path.join(d, f)
        rel = os.path.relpath(p, root)
        if '/test' in rel or rel.startswith('test') or 'tests.rs' in rel or 'anvil' in rel or 'wlcs' in rel: continue
        src = open(p, errors='replace').read()
        # cut off #[cfg(test)] mod tests { ... } (everything after the first #[cfg(test)] at column 0)
        i = src.find('\n#[cfg(test)]')
        if i >= 0: src = src[:i]
        for ln in src.split('\n'):
            t = ln.strip()
            if t.startswith('//'): continue
            counted = False
            for name, rx in pats.items():
                if rx.search(ln):
                    if name == 'other .unwrap()' and counted: continue
                    tot[name] += 1; files[rel.split('/')[0] + '/' + (rel.split('/')[1] if '/' in rel else '')][name] += 1
                    if name in ('lock().unwrap()  (poisoned-lock panic)', '.data::<..>().unwrap() / user_data().get().unwrap()  (set up by the same module)'): counted = True
for k, v in tot.items(): print(f'{v:6d}  {k}')
print('\nby top directory (unwrap + expect):')
agg = collections.Counter()
for dd, c in files.items(): agg[dd] = c['other .unwrap()'] + c['.expect('] + c['lock().unwrap()  (poisoned-lock panic)'] + c['.data::<..>().unwrap() / user_data().get().unwrap()  (set up by the same module)']
for k, v in agg.most_common(12): print(f'{v:6d}  {k}')
