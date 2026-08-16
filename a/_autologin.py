# -*- coding: utf-8 -*-
"""Auto: fetch captcha -> OCR -> login -> immediately crawl key data."""
import base64, json, re, sys, time, os, urllib.request, ssl
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives.serialization import load_der_public_key
from PIL import Image, ImageOps
import easyocr

ctx = ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
BASE = "https://www.aiitss.cn/prod-api"
PUB = "MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAKoR8mX0rGKLqzcWmOzbfj64K8ZIgOdH\nnzkXSOVOZbFu/TJhZ7rFAN+eaGkl3C4buccQd/EjEsj9ir7ijT7h96MCAwEAAQ=="
pubkey = load_der_public_key(base64.b64decode(PUB))
reader = easyocr.Reader(['en'], gpu=False, verbose=False)

def rsa_enc(s):
    return base64.b64encode(pubkey.encrypt(s.encode(), padding.PKCS1v15())).decode()

def req(path, body=None, token=None, method=None):
    url = BASE + path
    m = method or ('POST' if body is not None else 'GET')
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(url, data=data, method=m)
    r.add_header('Content-Type', 'application/json')
    if token: r.add_header('Authorization', 'Bearer ' + token)
    try:
        with urllib.request.urlopen(r, context=ctx, timeout=30) as resp:
            raw = resp.read().decode()
            try: return resp.status, json.loads(raw)
            except: return resp.status, {'raw': raw[:500]}
    except urllib.error.HTTPError as e:
        return e.code, {'httperr': e.read().decode()[:500]}

def fetch_captcha():
    st, j = req('/code')
    img = base64.b64decode(j['img']); open('_cap_cand.jpg','wb').write(img)
    return j['uuid']

def ocr_captcha():
    im = Image.open('_cap_cand.jpg').convert('L')
    im = im.resize((im.size[0]*6, im.size[1]*6), Image.LANCZOS)
    im = ImageOps.autocontrast(im); im.save('_cap_cand_big.png')
    res = reader.readtext('_cap_cand_big.png', detail=1, allowlist='0123456789+-xX*=')
    res = sorted(res, key=lambda d: d[0][0][0])
    if not res: return '', 0, []
    txt = ''.join(d[1] for d in res)
    conf = min(d[2] for d in res)
    return txt, conf, res

def compute(txt):
    t = txt.replace('X','*').replace('x','*').replace('=','')
    # simple arithmetic evaluation
    try:
        # find longest run of digit-op-digit pattern
        import ast, operator
        if re.fullmatch(r'[\d+\-*xX()]+', t):
            safe = t.replace('x','*').replace('X','*')
            # evaluate left-to-right via python with care for no negatives prefix issues
            val = eval(safe, {'__builtins__':{}}, {})
            return val
    except Exception:
        pass
    return None

def try_login(code, uuid):
    body = {"username":rsa_enc("13955832695"),"password":rsa_enc("zkla@2026"),
            "code":code,"uuid":uuid,"loginType":rsa_enc("false"),"smsCode":rsa_enc("")}
    st, res = req('/auth/login', body)
    return res

def main():
    os.makedirs('_crawled', exist_ok=True)
    best = None
    for i in range(6):
        uuid = fetch_captcha()
        txt, conf, items = ocr_captcha()
        print(f'[try {i}] uuid={uuid} ocr={txt!r} conf={conf:.2f}')
        if not txt: 
            time.sleep(0.4); continue
        if conf < 0.40:
            # low confidence; still keep as candidate but prefer higher
            pass
        val = compute(txt)
        if val is None: 
            print('   cannot compute'); continue
        print(f'   computed answer = {val}')
        # server timeout param if file modified
        res = try_login(str(val), uuid)
        print('   login resp code=', res.get('code'))
        if res.get('code') == 200 and res.get('data',{}).get('access_token'):
            token = res['data']['access_token']
            open('_token.txt','w').write(token)
            print('   TOKEN SAVED')
            # test
            st2, gi = req('/system/user/getInfo', token=token)
            print('   getInfo', st2, json.dumps(gi,ensure_ascii=False)[:150])
            if gi.get('code')==200:
                print('   LOGIN FULLY VERIFIED')
                return token
        time.sleep(0.5)
    print('Could not land a login in this batch')

if __name__ == '__main__':
    main()
