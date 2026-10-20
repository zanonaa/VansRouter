"""Inventário conservador: rotas Next e literais registrados pelo router Go.
Não conta nomes dinâmicos de loops como migrados sem revisão manual.
"""
from pathlib import Path
import re,json
root=Path(__file__).resolve().parents[2]
router=(root/'backend/internal/http/router/router.go').read_text()
registered=set()
for method,path in re.findall(r'r\.(Get|Post|Put|Patch|Delete|Head|Options)\("([^\"]+)"',router):
 registered.add((method.upper(),path))
# Subrouter keys é uma exceção explícita ao caminho relativo.
for method,path in list(registered):
 if path=='/':registered.add((method,'/api/keys'))
 if path=='/{id}':registered.add((method,'/api/keys/{id}'))
entries=[]
for file in sorted((root/'src/app/api').rglob('route.js')):
 path='/api/'+str(file.parent.relative_to(root/'src/app/api'))
 path=re.sub(r'\[([^\]]+)\]',r'{\1}',path)
 for method in re.findall(r'export\s+(?:async\s+)?function\s+(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\(',file.read_text()):
  entries.append({'method':method,'path':path,'source':str(file.relative_to(root)),'status':'registered-needs-contract-review' if (method,path) in registered else 'not-proven'})
output=root/'backend/PHASE3_INVENTORY.json'
output.write_text(json.dumps({'warning':'Static registration is not proof of full migration. Dynamic/public aliases need manual review.','contracts':entries},indent=2)+'\n')
print(f'{len(entries)} contratos; {sum(e["status"]=="not-proven" for e in entries)} sem registro comprovado')
