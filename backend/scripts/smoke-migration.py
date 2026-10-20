#!/usr/bin/env python3
"""Smoke TCP do binário Go com dados fictícios e diretório descartável.
Uso: python3 backend/scripts/smoke-migration.py /caminho/para/api
"""
import http.cookiejar
import json
import os
from pathlib import Path
import signal
import socket
import sqlite3
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request


def check(condition, message):
    if not condition:
        raise AssertionError(message)


def main():
    binary = str(Path(sys.argv[1]).resolve(strict=True))
    # O servidor suporta PORT, não porta zero. A pequena janela entre reserva
    # e bind é aceita no smoke; indisponibilidade resulta em falha explícita.
    with socket.socket() as reservation:
        reservation.bind(('127.0.0.1', 0))
        port = reservation.getsockname()[1]
    with tempfile.TemporaryDirectory(prefix='migration-smoke-') as tmp:
        directory = Path(tmp)
        legacy = {'settings': {'requireLogin': True},
                  'providerConnections': [{'id': 'legacy-connection', 'provider': 'openai',
                      'authType': 'apikey', 'name': 'Legacy', 'apiKey': 'fictitious-secret'}]}
        original = json.dumps(legacy)
        (directory / 'db.json').write_text(original)
        (directory / 'usage.json').write_text(json.dumps({'history': [{'tokens': {'input_tokens': 7}}]}))
        environment = {'PATH': os.environ.get('PATH', ''), 'HOME': tmp,
                       'DATA_DIR': tmp, 'PORT': str(port), 'NODE_ENV': 'development',
                       'JWT_SECRET': 'fictitious-smoke-secret', 'INITIAL_PASSWORD': '123456'}
        base = f'http://127.0.0.1:{port}'
        jar = http.cookiejar.CookieJar()
        client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))

        def request(method, path, payload=None):
            data = None if payload is None else json.dumps(payload).encode()
            req = urllib.request.Request(base + path, data=data, method=method,
                                         headers={'Content-Type': 'application/json'})
            try:
                with client.open(req, timeout=3) as response:
                    return response.status, json.load(response)
            except urllib.error.HTTPError as error:
                return error.code, json.load(error)

        def start(log):
            process = subprocess.Popen([binary], env=environment, stdout=log, stderr=log)
            try:
                deadline = time.monotonic() + 15
                while time.monotonic() < deadline:
                    if process.poll() is not None:
                        raise AssertionError('servidor encerrou durante startup; consulte log do smoke')
                    try:
                        if request('GET', '/api/ready')[0] == 200:
                            return process
                    except (OSError, urllib.error.URLError):
                        pass
                    time.sleep(0.05)
                raise AssertionError('timeout de readiness')
            except BaseException:
                stop(process)
                raise

        def stop(process):
            if process.poll() is None:
                process.send_signal(signal.SIGTERM)
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)

        with (directory / 'server.log').open('w+') as log:
            process = start(log)
            try:
                check(request('GET', '/api/providers')[0] == 401, 'guard sem sessão')
                status, body = request('POST', '/api/auth/login', {'password': '123456'})
                check(status == 200 and body['success'], 'login')
                status, body = request('GET', '/api/providers/legacy-connection')
                check(status == 200 and 'apiKey' not in body['connection'], 'import/omissão de segredo')
                check(request('PUT', '/api/providers/legacy-connection', {'name': 'Persisted'})[0] == 200, 'update')
                check((directory / 'db' / '.migrated-from-json').exists(), 'marcador de importação')
            finally:
                stop(process)
            before = sorted(str(p.relative_to(directory)) for p in (directory / 'db' / 'backups').rglob('*'))
            process = start(log)
            try:
                status, body = request('GET', '/api/providers/legacy-connection')
                check(status == 200 and body['connection']['name'] == 'Persisted', 'sessão/persistência após restart')
            finally:
                stop(process)
            after = sorted(str(p.relative_to(directory)) for p in (directory / 'db' / 'backups').rglob('*'))
            check(before == after, 'backups extras no restart')
            with sqlite3.connect(directory / 'db' / 'data.sqlite') as db:
                check(db.execute('SELECT count(*) FROM usageHistory').fetchone()[0] == 1, 'histórico duplicado')
                check(db.execute("SELECT apiKey FROM (SELECT json_extract(data,'$.apiKey') AS apiKey FROM providerConnections)").fetchone()[0] == 'fictitious-secret', 'credencial perdida')
            check((directory / 'db.json').read_text() == original, 'arquivo legado alterado')
        print('PASS: TCP/importação/login/CRUD/restart/sessão/histórico/backups/original')


if __name__ == '__main__':
    main()
