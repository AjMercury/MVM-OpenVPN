#!/usr/bin/env python3
"""Disposable-host smoke test; password only on protected stdin, never output."""
import json
import pathlib
import ssl
import sys
import urllib.error
import urllib.request

password = sys.stdin.read().strip()
address, username = sys.argv[1:3]
config = dict(line.split('=', 1) for line in pathlib.Path('/etc/mvm/config.env').read_text().splitlines() if '=' in line)
assert config['MVM_LISTEN'].startswith('0.0.0.0:'), 'listener must accept server interface addresses'
assert config['MVM_DEV_HTTP'] == 'false'
scheme = config['MVM_ORIGIN'].split('://')[0]
assert scheme == 'https' or config.get('MVM_ALLOW_HTTP') == 'true'
port = config['MVM_LISTEN'].rsplit(':', 1)[1]
context = ssl.create_default_context(cafile=config.get('MVM_TLS_CA_FILE'))
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPSHandler(context=context))
origins = [config['MVM_ORIGIN']] + [s for s in config.get('MVM_ADDITIONAL_ORIGINS', '').split(',') if s]
for origin in origins:
    request = urllib.request.Request(f'{scheme}://{address}:{port}/login',
        data=json.dumps({'username':username,'password':password}).encode(),
        headers={'Origin':origin,'Host':origin.split('://',1)[1],'Content-Type':'application/json'})
    with opener.open(request, timeout=10) as response:
        assert response.status == 200
        cookie = response.headers['Set-Cookie']
        assert 'HttpOnly' in cookie and ('Secure' in cookie) == (scheme == 'https')
        csrf = json.load(response)['csrf']
    request = urllib.request.Request(f'{scheme}://{address}:{port}/api/v1/logout', data=b'{}',
        headers={'Origin':origin,'Cookie':cookie.split(';',1)[0],'X-CSRF-Token':csrf,'Content-Type':'application/json'})
    with opener.open(request, timeout=10) as response:
        assert response.status == 200
try:
    opener.open(f'{scheme}://{address}:{port}/setup', timeout=10)
except urllib.error.HTTPError as exc:
    assert exc.code == 404
else:
    raise SystemExit('Browser setup remained enabled')
print(f'PASS: direct {scheme} IP/domain login, correct cookie security, CSRF logout and closed setup')
