# Bulk download all chunk JS files listed in _all_chunks.txt, skip existing
import os, subprocess, sys

base = os.path.dirname(os.path.abspath(__file__))
outdir = os.path.join(base, '_chunks_all')
os.makedirs(outdir, exist_ok=True)
urls = [l.strip() for l in open(os.path.join(base,'_all_chunks.txt')) if l.strip()]
print('total to download:', len(urls))
ok = skip = fail = 0
with open(os.path.join(base,'_chunk_download_log.txt'),'w') as log:
    for u in urls:
        target = os.path.join(outdir, u)
        if os.path.exists(target) and os.path.getsize(target) > 0:
            skip += 1; continue
        r = subprocess.run(['curl.exe','-s','-k', 'https://www.aiitss.cn/static/js/'+u, '-o', target],
                           capture_output=True, timeout=30)
        if os.path.exists(target) and os.path.getsize(target) > 500:
            ok += 1
        else:
            fail += 1
            log.write(u + '\n')
print(f'ok={ok} skip={skip} fail={fail}')
