# -*- coding: utf-8 -*-
"""Crawl authenticated data structures immediately after login. Run fast, token ~12min."""
import base64, json, os, time, urllib.request, ssl
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives.serialization import load_der_public_key
from PIL import Image, ImageOps
import easyocr

ctx = ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
BASE = "https://www.aiitss.cn/prod-api"
PUB = "MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAKoR8mX0rGKLqzcWmOzbfj64K8ZIgOdH\nnzkXSOVOZbFu/TJhZ7rFAN+eaGkl3C4buccQd/EjEsj9ir7ijT7h96MCAwEAAQ=="
pubkey = load_der_public_key(base64.b64decode(PUB))
reader = easyocr.Reader(['en'], gpu=False, verbose=False)

def rsa_enc(s): return base64.b64encode(pubkey.encrypt(s.encode(), padding.PKCS1v15())).decode()

def req(path, body=None, token=None, method=None):
    url = BASE + path
    m = method or ('POST' if body is not None else 'GET')
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(url, data=data, method=m)
    r.add_header('Content-Type','application/json')
    if token: r.add_header('Authorization','Bearer '+token)
    try:
        with urllib.request.urlopen(r, context=ctx, timeout=30) as resp:
            raw = resp.read().decode()
            try: return resp.status, json.loads(raw)
            except: return resp.status, {'raw': raw[:600]}
    except urllib.error.HTTPError as e:
        return e.code, {'httperr': e.read().decode()[:600]}
    except Exception as e:
        return -1, {'ex': str(e)}

def ensure_login():
    # log in using a high-confidence OCR captcha
    for i in range(12):
        st, j = req('/code')
        open('_cap_tmp.jpg','wb').write(base64.b64decode(j['img']))
        im = Image.open('_cap_tmp.jpg').convert('L'); im=im.resize((im.size[0]*6,im.size[1]*6),Image.LANCZOS); im=ImageOps.autocontrast(im); im.save('_cap_tmp_big.png')
        res = reader.readtext('_cap_tmp_big.png', detail=1, allowlist='0123456789+-xX*=')
        res = sorted(res, key=lambda d:d[0][0][0])
        if not res: continue
        txt = ''.join(d[1] for d in res); conf = min(d[2] for d in res)
        if conf < 0.7: continue
        import re
        t = txt.replace('X','*').replace('x','*').replace('=','')
        m = re.search(r'\d+([+\-*]\d+)*', t)
        expr = m.group(0) if m else ''
        try:
            val = eval(expr)
        except Exception:
            continue
        body = {"username":rsa_enc("13955832695"),"password":rsa_enc("zkla@2026"),
                "code":str(val),"uuid":j["uuid"],"loginType":rsa_enc("false"),"smsCode":rsa_enc("")}
        st2, rj = req('/auth/login', body)
        if rj.get('code')==200 and rj.get('data',{}).get('access_token'):
            tok = rj['data']['access_token']
            open('_token.txt','w').write(tok)
            print('LOGIN OK. token saved. ocr=',txt,'ans=',val)
            return tok
    return None

def main():
    os.makedirs('_crawled', exist_ok=True)
    token = ensure_login()
    if not token:
        print('LOGIN FAILED'); return
    t0=time.time()
    endpoints = [
        ('01_user_getInfo', '/system/user/getInfo'),
        ('02_menu_getRouters', '/system/menu/getRouters'),
        ('03_user_list', '/system/user/list?pageNum=1&pageSize=10'),
        ('04_dict_type_list', '/system/dict/type/list?pageNum=1&pageSize=100'),
        ('05_role_list', '/system/role/list?pageNum=1&pageSize=50'),
        ('06_dept_list', '/system/dept/list'),
        ('07_config_list', '/system/config/list?pageNum=1&pageSize=100'),
    ]
    for name, ep in endpoints:
        st, j = req(ep, token=token)
        fn = f'_crawled/{name}.json'
        open(fn,'w',encoding='utf-8').write(json.dumps({'status':st,'data':j},ensure_ascii=False,indent=2))
        head = json.dumps(j,ensure_ascii=False)[:140]
        print(f'{name}: st={st} {head}')
        time.sleep(0.2)
    print('ELAPSED %.1fs' % (time.time()-t0))

if __name__=='__main__':
    main()
