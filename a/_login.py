# -*- coding: utf-8 -*-
import base64, json, subprocess, urllib.request, ssl, sys
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives.serialization import load_der_public_key

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

PUB = "MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAKoR8mX0rGKLqzcWmOzbfj64K8ZIgOdH\nnzkXSOVOZbFu/TJhZ7rFAN+eaGkl3C4buccQd/EjEsj9ir7ijT7h96MCAwEAAQ=="
pub_der = base64.b64decode(PUB)
pubkey = load_der_public_key(pub_der)

def rsa_enc(s: str) -> str:
    b = s.encode('utf-8')
    ct = pubkey.encrypt(b, padding.PKCS1v15())
    return base64.b64encode(ct).decode()

def api(path, body=None, token=None):
    url = "https://www.aiitss.cn/prod-api" + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method='POST' if body is not None else 'GET')
    req.add_header('Content-Type', 'application/json')
    if token:
        req.add_header('Authorization', 'Bearer ' + token)
    with urllib.request.urlopen(req, context=ctx, timeout=30) as r:
        return json.loads(r.read().decode())

def get_code():
    with urllib.request.urlopen("https://www.aiitss.cn/prod-api/code", context=ctx, timeout=30) as r:
        j = json.loads(r.read().decode())
    img = base64.b64decode(j['img'])
    open('_cf_challenge.jpg','wb').write(img)
    return j['uuid'], j['img']

if __name__ == '__main__':
    username = sys.argv[1]
    password = sys.argv[2]
    code = sys.argv[3]
    uuid = sys.argv[4]
    body = {
        "username": rsa_enc(username),
        "password": rsa_enc(password),
        "code": code,
        "uuid": uuid,
        "loginType": rsa_enc("false"),
        "smsCode": rsa_enc(""),
    }
    print("REQUEST:", json.dumps(body)[:200])
    resp = api('/auth/login', body)
    print("RESPONSE:", json.dumps(resp, ensure_ascii=False)[:300])
