# Extract field names from a Vue view chunk: look for data(){return{...}} and form objects
import os, re, sys

base = os.path.dirname(os.path.abspath(__file__))
f = sys.argv[1]
c = open(os.path.join(base, f), encoding='utf-8', errors='ignore').read()

print('### file:', f, 'len', len(c))

# Find data objects: patterns like  xx:{...},  h:"id", etc. Look for form model definitions
# Common: form:{field:"",field2:0,...} or form:{ field:"" }
# Search  {form:{ ... }} block
m = re.search(r'form:\{', c)
if m:
    # capture up to reasonable depth
    i = m.end()-1  # at '{'
    depth=1; j=i+1
    while depth>0 and j < len(c):
        if c[j]=='{': depth+=1
        elif c[j]=='}': depth-=1
        j+=1
    block = c[i:j]
    keys = re.findall(r'([A-Za-z_$][\w$]*)\s*:', block)
    # filter out obvious non-fields
    print('--- form object keys ---')
    print(', '.join(keys[:60]))

# Also look for any { key:"" , key:0 } table query structures
m2 = re.search(r'(queryParams|listQuery)\s*:\s*\{', c)
if m2:
    i=c.find('{', m2.start()); depth=1; j=i+1
    while depth>0 and j<len(c):
        if c[j]=='{':depth+=1
        elif c[j]=='}':depth-=1
        j+=1
    keys=re.findall(r'([A-Za-z_$][\w$]*)\s*:', c[i:j])
    print('--- listQuery/queryParams keys ---')
    print(', '.join(keys[:60]))
