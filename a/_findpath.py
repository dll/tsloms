# Find which chunk files reference a given API path, then extract nearby data() object
import os, re, glob, sys

base = os.path.dirname(os.path.abspath(__file__))
target = sys.argv[1]  # e.g. '/data/dataPoint/add'
files = glob.glob(os.path.join(base, '_chunks_all', '*.js'))
hits = []
for f in files:
    c = open(f, encoding='utf-8', errors='ignore').read()
    if target in c:
        hits.append(os.path.basename(f))
print('FILES with', target, ':', hits)
