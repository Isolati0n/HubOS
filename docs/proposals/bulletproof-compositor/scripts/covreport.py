#!/usr/bin/env python3
"""covreport.py PROTO_DIR OUT.md JSON... : merge fuzz2 coverage files; which requests of the advertised protocols were sent."""
import glob, json, sys, collections
import xml.etree.ElementTree as ET
proto, out = sys.argv[1], sys.argv[2]
sent = collections.Counter(); globs = {}
files = []
for pat in sys.argv[3:]: files += glob.glob(pat)
errors = collections.Counter()
for f in files:
    d = json.load(open(f)); sent.update(d['sent'])
    for g, v in d.get('globals', []): globs[g] = max(globs.get(g, 0), v)
    errors.update(d.get('errors', {}))
ifaces = {}
for f in glob.glob(proto + '/*.xml'):
    try: root = ET.parse(f).getroot()
    except Exception: continue
    for i in root.iter('interface'):
        reqs = []
        for m in i.findall('request'):
            news = [a.get('interface') for a in m.findall('arg') if a.get('type') == 'new_id' and a.get('interface')]
            reqs.append((m.get('name'), news))
        ifaces[i.get('name')] = reqs
# closure of interfaces reachable from the advertised globals via new_id arguments
reach = set(); todo = [g for g in globs if g in ifaces]
while todo:
    g = todo.pop()
    if g in reach: continue
    reach.add(g)
    for name, news in ifaces[g]:
        todo += [n for n in news if n in ifaces]
# server-created objects (events with new_id) are not requests; ignore.
rows = []; tot = 0; cov1 = 0; cov20 = 0
never = []
for i in sorted(reach):
    for name, _ in ifaces[i]:
        k = f'{i}.{name}'; n = sent.get(k, 0); tot += 1; cov1 += n >= 1; cov20 += n >= 20
        if n == 0: never.append(k)
        rows.append((i, name, n))
with open(out, 'w') as o:
    o.write(f'files merged: {len(files)}; advertised globals seen: {len(globs)}; interfaces reachable (globals plus objects they create): {len(reach)}\n')
    o.write(f'requests in those interfaces: {tot}; sent at least once: {cov1} ({100*cov1/tot:.0f}%); sent at least 20 times: {cov20} ({100*cov20/tot:.0f}%); never sent: {tot-cov1}\n\n')
    o.write('advertised globals (name version): ' + ', '.join(f'{g} v{v}' for g, v in sorted(globs.items())) + '\n\n')
    o.write('never sent:\n' + '\n'.join('  ' + k for k in never) + '\n\n')
    o.write('protocol errors that disconnected a fuzz client (count):\n' + '\n'.join(f'  {c:6d} {k}' for k, c in errors.most_common(40)) + '\n')
print(open(out).read()[:3000])
