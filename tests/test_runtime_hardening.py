#!/usr/bin/env python3
"""Runtime contracts without Docker or third-party Python dependencies."""
import json
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]
TEMPLATES = ['frontend-template','backend-template','fullstack-simple','fullstack-unified','fullstack-showcase','fullstack-chat','fullstack-oidc']
FRONTENDS = ['frontend-template'] + [f'fullstack-{n}/frontend' for n in ['simple','showcase','chat','oidc']]
GO = ['backend-template','fullstack-unified'] + [f'fullstack-{n}/backend' for n in ['simple','showcase','chat','oidc']]

def read(p): return (ROOT / p).read_text()

class Hardening(unittest.TestCase):
 def test_frontend_runtime(self):
  for f in FRONTENDS:
   with self.subTest(frontend=f):
    self.assertIn('USER 101:101', read(f+'/Dockerfile'))
    c=read(f+'/nginx.conf'); e=read(f+'/entrypoint.sh')
    for v in ['listen 8080;', 'pid /tmp/nginx.pid;', 'server_tokens off;', 'Referrer-Policy "strict-origin-when-cross-origin" always;', 'client_body_temp_path /tmp/client_temp;', 'root /tmp/html;']:
     self.assertIn(v,c)
    self.assertNotIn('user nginx;',c)
    self.assertIn('> /tmp/nginx.conf',e)
    self.assertIn('nginx -c /tmp/nginx.conf',e)
    self.assertIn('cp -R /usr/share/nginx/html/. /tmp/html/',e)
    self.assertIn('INDEX_HTML="/tmp/html/index.html"',e)
    # Only content-hashed Vite assets get immutable caching; all other
    # files (including public/*.js and HTML) must revalidate.
    loc=re.search(r'location ~\* (.*?) \{\s*expires 1y;', c).group(1)
    pattern=loc.strip('"')
    for path,expected in [('/assets/index-Ab12_cd3.js',True),('/assets/logo-12345678.svg',True),('/assets/plain.js',False),('/public/app.js',False),('/logo.svg',False),('/index.html',False)]:
     self.assertEqual(bool(re.search(pattern,path)),expected,(f,path))
    self.assertIn('add_header Cache-Control "no-cache";',c)
 def test_upload_configuration_and_docs(self):
  for f in FRONTENDS:
   with self.subTest(frontend=f):
    self.assertIn('client_max_body_size ${MOSES_MAX_BODY_BYTES};', read(f+'/nginx.conf'))
    template=f.removesuffix('/frontend')
    self.assertIn('MOSES_MAX_BODY_BYTES', read(template+'/README.md'))
    values=read(template+'/helm/values.yaml')
    self.assertIn('MOSES_MAX_BODY_BYTES: "1048576"', values)
    if '/frontend' in f:
     self.assertIn('hasKey (.env | default dict) "MOSES_MAX_BODY_BYTES"', read(template+'/helm/templates/deployment.yaml'))
     self.assertIn('fail "move MOSES_MAX_BODY_BYTES to globalEnv: one value now applies to the frontend proxy and the backend"', read(template+'/helm/templates/deployment.yaml'))
     self.assertIn('globalEnv.MOSES_MAX_BODY_BYTES', read(template+'/README.md'))
 def test_redis_enablement_docs(self):
  for term in ['redis.auth.password', 'REDIS_PASSWORD', 'coordinated', '__MOSES_GENERATE_PASSWORD__']:
   self.assertIn(term, read('fullstack-simple/README.md'))
 def test_go_wiring_and_ignores(self):
  for g in GO:
   with self.subTest(module=g):
    main=read(g+('/main.go' if g=='fullstack-unified' else '/cmd/server/main.go'))
    self.assertIn('LimitRequestBody(',main)
    self.assertIn('buildHandler(', main)
    server=g if g=='fullstack-unified' else g+'/cmd/server'
    self.assertIn('testStreaming(t,', read(server+'/streaming_chain_test.go'))
    self.assertIn('RejectCrossSiteCSRF(',main)
    self.assertIn('.env*',read(g+'/.dockerignore').splitlines())
  self.assertIn('fullstack-oidc/backend/internal/middleware/csrf.go',read('tools/check-vendored-csrf.sh'))
 def test_identity_and_errors(self):
  for f in ['backend-template/internal/middleware/moses_headers.go','fullstack-showcase/backend/internal/middleware/moses_headers.go','fullstack-chat/backend/internal/middleware/moses_headers.go','fullstack-unified/main.go']:
   self.assertRegex(read(f),r'UntrustedUserID\s+string')
   self.assertNotRegex(read(f),r'\bUserID\s+string')
  for f in ['shared/mosesproxy-go/proxy.go','fullstack-chat/backend/internal/handler/webhook_chat.go']:
   self.assertNotRegex(read(f),r'write(?:Err|Error)\([^\n]*err.Error\(\)')
 def test_chart_contracts(self):
  for t in TEMPLATES:
   with self.subTest(chart=t):
    v=read(t+'/helm/values.yaml'); d=read(t+'/helm/templates/deployment.yaml')
    for x in ['podSecurityContext:', 'securityContext:', 'runAsNonRoot: true','runAsUser:', 'runAsGroup:', 'fsGroup:', 'allowPrivilegeEscalation: false','readOnlyRootFilesystem: true','drop: [ALL]', 'type: RuntimeDefault', 'automountServiceAccountToken: false']:
     self.assertIn(x,v)
    for x in ['podSecurityContext','securityContext','automountServiceAccountToken','emptyDir: {}','mountPath: /tmp']:
     self.assertIn(x,d)
    self.assertIn('maintenance',json.loads(read(t+'/moses-app.config.json')))
  for t in ['fullstack-chat','fullstack-showcase','fullstack-oidc']:
   self.assertIn('tag: "17-alpine"',read(t+'/helm/values.yaml'))
   p=read(t+'/helm/templates/postgresql.yaml')
   for x in ['fail ', '__MOSES_GENERATE_PASSWORD__', 'postgresql.podSecurityContext','postgresql.securityContext','mountPath: /var/run/postgresql','mountPath: /tmp']:
    self.assertIn(x,p)
  r=read('fullstack-simple/helm/templates/redis.yaml')
  for x in ['fail ', '__MOSES_GENERATE_PASSWORD__','secretKeyRef:', '--requirepass','redis.podSecurityContext','redis.securityContext','mountPath: /data']:
   self.assertIn(x,r)
  self.assertIn('name: REDIS_PASSWORD',read('fullstack-simple/helm/templates/deployment.yaml'))

if __name__ == '__main__': unittest.main()
