# -*- coding: utf-8 -*-
"""Crawl signal-module endpoints with valid token."""
import json, os, time, urllib.request, ssl

ctx = ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
BASE = "https://www.aiitss.cn/prod-api"
TOKEN = open('_token.txt').read().strip()

def req(path, body=None, method=None):
    url = BASE + path
    m = method or ('POST' if body is not None else 'GET')
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(url, data=data, method=m)
    r.add_header('Content-Type','application/json')
    r.add_header('Authorization','Bearer '+TOKEN)
    try:
        with urllib.request.urlopen(r, context=ctx, timeout=25) as resp:
            try: return resp.status, json.loads(resp.read().decode())
            except: return resp.status, None
    except urllib.error.HTTPError as e:
        return e.code, {'httperr': e.read().decode()[:400]}
    except Exception as e:
        return -1, {'ex':str(e)}

os.makedirs('_crawled', exist_ok=True)
t0=time.time()
endpoints = [
    ('20_crossing_crossList','/signal/crossing/crossList?pageNum=1&pageSize=20'),
    ('21_crossing_list','/signal/crossing/list?pageNum=1&pageSize=20'),
    ('22_warning_list','/signal/warning/list?pageNum=1&pageSize=20'),
    ('23_equipment_list','/signal/equipment/list?pageNum=1&pageSize=20'),
    ('24_warnconfig_list','/signal/warning/config/list?pageNum=1&pageSize=20'),
    ('25_map_getData','/signal/crossingMap/getMapData'),
    ('26_baseData_areas','/baseData/api/areas'),
    ('27_extra_list','/signal/extra/list?pageNum=1&pageSize=20'),
    ('28_crossing_pointList','/signal/crossing/pointList?pageNum=1&pageSize=20'),
    ('29_dataPoint_listByCondition','/data/dataPoint/listByCondition?pageNum=1&pageSize=20'),
]
for name, ep in endpoints:
    st, j = req(ep)
    open(f'_crawled/{name}.json','w',encoding='utf-8').write(json.dumps({'status':st,'body':j},ensure_ascii=False,indent=2))
    head = json.dumps(j,ensure_ascii=False)[:150] if j else 'null'
    print(f'{name}: st={st} {head}')
    time.sleep(0.15)
print('ELAPSED %.1fs' % (time.time()-t0))
