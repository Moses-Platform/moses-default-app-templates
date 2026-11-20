#!/usr/bin/env python3
"""Execute the real entrypoint against immutable fixture inputs; stub only nginx.
Docker must separately prove UID, read-only mounts and nginx HTTP behavior.
"""
import os
from pathlib import Path
import subprocess
import sys
import tempfile

template, framing, ancestors, report, output, base, domain = sys.argv[1:8]
body_limit = sys.argv[8] if len(sys.argv) > 8 else ""
p=Path(template)
with tempfile.TemporaryDirectory() as tmp:
 root=Path(tmp)
 for d in ['etc/nginx','usr/share/nginx/html/assets','tmp','bin']:
  (root/d).mkdir(parents=True,exist_ok=True)
 source=root/'usr/share/nginx/html/index.html'
 original='<meta name="moses-base-path" content="__MOSES_BASE_PATH__">'
 source.write_text(original)
 (source.parent/'assets/index-Ab12_cd3.js').write_text('asset')
 config=root/'etc/nginx/nginx.conf.template'
 config.write_text(p.read_text())
 stub=root/'bin/nginx'
 stub.write_text('#!/bin/sh\n[ "$1" = "-c" ] && [ -s "$2" ] && [ "$3" = "-g" ] && [ "$4" = "daemon off;" ]\n')
 stub.chmod(0o755)
 script=(p.parent/'entrypoint.sh').read_text()
 for path in ['/tmp/', '/etc/nginx/', '/usr/share/nginx/html']:
  script=script.replace(path,str(root)+path)
 entry=root/'entrypoint.sh'; entry.write_text(script)
 for f in [source, config]: f.chmod(0o444)
 for d in [source.parent,config.parent]: d.chmod(0o555)
 env=dict(os.environ,PATH=str(root/'bin')+':'+os.environ['PATH'],MOSES_BASE_PATH=base,MOSES_EMBEDDING_FRAMING=framing,MOSES_EMBEDDING_ALLOWED_ANCESTORS=ancestors,MOSES_EMBEDDING_REPORT_URI=report,MOSES_DOMAIN=domain,MOSES_MAX_BODY_BYTES=body_limit,BACKEND_SERVICE_HOST='example-backend',BACKEND_SERVICE_PORT='8080')
 try:
  result = subprocess.run(['sh',str(entry)],env=env,check=True,capture_output=True,text=True)
  Path(output+'.log').write_text(result.stdout+result.stderr)
  rendered=root/'tmp/html/index.html'
  assert source.read_text()==original, 'image asset changed'
  assert rendered.read_text()==original.replace('__MOSES_BASE_PATH__',base)
  assert rendered.stat().st_mode & 0o777 == 0o644
  assert (root/'tmp/html/assets/index-Ab12_cd3.js').read_text()=='asset'
  Path(output).write_text((root/'tmp/nginx.conf').read_text())
 finally:
  for d in [source.parent,config.parent]: d.chmod(0o755)
