# Extract all API endpoints (url:"...") across chunks -> write _api_inventory.txt
import os, re, glob
from collections import defaultdict, OrderedDict

base = os.path.dirname(os.path.abspath(__file__))
files = glob.glob(os.path.join(base, '_chunks_all', '*.js'))
api_map = OrderedDict()
pat1 = re.compile(r'url:\s*"([^"]+)"')
pat2 = re.compile(r'url:\s*`([^`]+)`')

def is_param(s):
    # template-like urls with ${...} -> record as pattern
    return '${' in s

for f in files:
    name = os.path.basename(f)
    try:
        c = open(f, encoding='utf-8', errors='ignore').read()
    except Exception:
        continue
    seen = set()
    for m in pat1.finditer(c):
        u = m.group(1)
        if u.startswith('/') and u not in seen:
            seen.add(u); api_map.setdefault(u, set()).add(name)
    for m in pat2.finditer(c):
        u = m.group(1)
        if (u.startswith('/') or u.startswith('`/')) and u not in seen:
            seen.add(u); api_map.setdefault(u, set()).add(name)

# build text
lines = []
for u, fl in api_map.items():
    # fl is set of module ids; map to fulling chunk filename via a registry
    lines.append(f'{u}\t{','.join(sorted(fl))}')
open(os.path.join(base,'_api_inventory.txt'),'w',encoding='utf-8').write('\n'.join(lines))

# summary by top-level
from collections import defaultdict
groups = defaultdict(int)
for u in api_map:
    top = u.split('/')[1] if u.count('/')>=1 else '(root)'
    groups[top] += 1
print('TOTAL unique API endpoints:', len(api_map))
for g in sorted(groups):
    print(f'/{g}: {groups[g]}')
