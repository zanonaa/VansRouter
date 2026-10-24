# Provider suggested-models

GET /api/providers/suggested-models portado com filtros openrouter-free, opencode-free, opencode-go, mimo-free e nvidia-free. Parâmetros ausentes/tipo desconhecido 400; redirecionamento e upstream não-2xx 502; timeout 15s. Guard de destino bloqueia nomes internos e ranges IPv4/IPv6 privados, valida todos os resultados DNS e fixa o primeiro endereço validado no dialer (sem proxy de ambiente nem redirect).

Testes sem rede: filtro de preços/contexto, redirects e ranges privados/mapeados. Suíte completa, vet, build e race aprovados em sandbox.

Divergências: parser JSON e URL produzem textos de erro próprios do Go. IPv6 link-local é bloqueado pelo range completo do Go (mais abrangente que o prefixo textual fe80: da origem). Casos malformados de modelos são ignorados em vez de disparar erros por acesso a propriedades como no JS.
