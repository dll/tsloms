# -*- coding: utf-8 -*-
"""robust login loop — prints real failure msg, tries more captchas."""
import base64, json, re, urllib.request, ssl
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

def req(path, body=None, token=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(BASE+path, data=data, method='POST' if body is not None else 'GET')
    r.add_header('Content-Type','application/json')
    if token: r.add_header('Authorization','Bearer '+token)
    try:
        with urllib.request.urlopen(r, context=ctx, timeout=30) as resp:
            return resp.status, json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()
    except Exception as e:
        return -1, {'ex': str(e)}

def main():
    for i in range(20):
        st, j = req('/code')
        open('_cap_tmp.jpg','wb').write(base64.b64decode(j['img']))
        im = Image.open('_cap_tmp.jpg').convert('L')
        im = im.resize((im.size[0]*7, im.size[1]*7), Image.LANCZOS); im=ImageOps.autocontrast(im)
        im.save('_cap_tmp_big.png')
        res = reader.readtext('_cap_tmp_big.png', detail=1, allowlist='0123456789+-xX*=')
        res = sorted(res, key=lambda d:d[0][0][0])
        if not res:
            continue
        txt = ''.join(d[1] for d in res); conf = min(d[2] for d in res)
        t = txt.replace('X','*').replace('x','*').replace('=','')
        m = re.search(r'\d+([+\-*]\d+)+', t)
        expr = m.group(0) if m else t
        try:
            val = eval(expr)
        except Exception:
            val = None
        print(f'[{i}] conf={conf:.2f} ocr={txt!r} expr={expr!r} val={val}')
        if val is None:
            continue
        if conf < 0.55:
            continue
        body = {"username":rsa_enc("13955832695"),"password":rsa_enc("zkla@2026"),
                "code":str(val),"uuid":j["uuid"],"loginType":rsa_enc("false"),"smsCode":rsa_enc("")}
        st2, rj = req('/auth/login', body)
        print(f'    login status={st2} code={rj.get("code") if isinstance(rj,dict) else rj} msg={rj.get("msg") if isinstance(rj,dict) else ""}')
        if isinstance(rj, dict) and rj.get('code')==200 and rj.get('data',{}).get('access_token'):
            open('_token.txt','w').write(rj['data']['access_token'])
            print('TOKEN SAVED, expires_in=', rj['data'].get('expires_in'))
            return
    print('NO LOGIN')

if __name__=='__main__':
    main()
