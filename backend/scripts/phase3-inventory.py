"""Inventário conservador: rotas Next e literais registrados pelo router Go.
Não conta nomes dinâmicos de loops como migrados sem revisão manual.
"""
from pathlib import Path
import re,json
root=Path(__file__).resolve().parents[2]
router=(root/'backend/internal/http/router/router.go').read_text()
registered=set()
pending=set()
for method,path,handler in re.findall(r'r\.(Get|Post|Put|Patch|Delete|Head|Options)\("([^\"]+)"\s*,\s*([^\n]+)',router):
 contract=(method.upper(),path)
 registered.add(contract)
 if 'handlers.Pending' in handler or '.NotPorted' in handler:
  pending.add(contract)
# Subrouter keys é uma exceção explícita ao caminho relativo.
for method,path in list(registered):
 if path=='/':registered.add((method,'/api/keys'))
 if path=='/{id}':registered.add((method,'/api/keys/{id}'))
gap_path=root/'backend/PHASE3_GAPS.json'
gaps=json.loads(gap_path.read_text())['gaps'] if gap_path.exists() else []
known_gaps={}
for gap in gaps:
 for path in gap['paths']:
  known_gaps.setdefault(path,[]).append(gap['id'])
entries=[]
for file in sorted((root/'src/app/api').rglob('route.js')):
 path='/api/'+str(file.parent.relative_to(root/'src/app/api'))
 path=re.sub(r'\[([^\]]+)\]',r'{\1}',path)
 for method in re.findall(r'export\s+(?:async\s+)?function\s+(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\(',file.read_text()):
   status='pending-implementation' if (method,path) in pending else ('registered-needs-contract-review' if (method,path) in registered else 'not-proven')
   issues=known_gaps.get(path,[]) if method not in ('OPTIONS','HEAD') else []
   if issues and status!='not-proven': status='partial-known-gaps'
   entry={'method':method,'path':path,'source':str(file.relative_to(root)),'status':status}
   if issues: entry['knownGaps']=issues
   entries.append(entry)
output=root/'backend/PHASE3_INVENTORY.json'
output.write_text(json.dumps({'warning':'Static registration is not proof of full migration. Dynamic/public aliases need manual review.','contracts':entries},indent=2)+'\n')
print(f'{len(entries)} contratos; {sum(e["status"]=="not-proven" for e in entries)} sem registro; {sum(e["status"]=="pending-implementation" for e in entries)} handlers pendentes; {sum(e["status"]=="partial-known-gaps" for e in entries)} contratos com lacunas conhecidas; nenhum contrato certificado apenas por registro')
