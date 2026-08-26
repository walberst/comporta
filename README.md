# Comporta

Gateway de API para empresas que precisam expor sistemas legados internos a parceiros
e aplicativos proprios sem abrir mao de autenticacao, controle de uso e visibilidade
sobre quem esta batendo em que rota.

## O problema

Toda empresa de porte medio, em algum momento, chega numa situacao parecida: existe um
sistema de faturamento, um de estoque, um de CRM, cada um rodando isolado dentro da
rede interna. Ai aparece um parceiro comercial, ou um app proprio, que precisa consumir
esses dados. A solucao mais rapida costuma ser abrir uma porta no firewall e distribuir
uma credencial fixa, e isso funciona, ate o dia em que um parceiro martela a API com
rajadas descontroladas, ou uma credencial vaza e ninguem sabe quais sistemas ela alcanca,
ou o time de suporte perde uma hora tentando entender por que uma integracao caiu sem
nenhum log de auditoria pra consultar.

O Comporta resolve isso colocando uma camada unica na frente de tudo: cada parceiro
recebe uma api key, cada rota fica associada a um servico upstream especifico, cada
combinacao parceiro/rota tem uma cota de requisicoes por minuto, e toda chamada fica
registrada. O sistema legado continua exatamente onde estava, o gateway e que decide
quem entra, quanto pode consumir, e guarda o rastro de tudo.

## Decisoes de arquitetura

**Por que Go.** Um gateway fica no caminho critico de toda requisicao que passa pela
empresa, entao latencia e uso de memoria previsivel importam mais do que produtividade
de desenvolvimento. Go compila para um binario estatico unico, tem concorrencia nativa
sem a complexidade de threads e locks manuais, e a biblioteca padrao ja resolve reverse
proxy HTTP (`net/http/httputil`) sem depender de nenhum framework pesado. Nao existe
ORM, nao existe injecao de dependencia magica: o `cmd/gateway/main.go` monta as pecas
na mao e fica claro o que depende do que.

**Roteamento por prefixo de path.** Cada rota cadastrada mapeia um prefixo (`/billing`,
`/inventory`) para uma URL de upstream. O gateway mantem essa tabela em memoria e
recarrega do Oracle a cada poucos segundos, em vez de consultar o banco a cada
requisicao: isso mantem o caminho quente do proxy livre de qualquer latencia de banco.
Quando dois prefixos colidem (`/billing` e `/billing/v2`), o mais especifico sempre
vence.

**Rate limiting com janela deslizante no Redis.** Um contador fixo por minuto (zera no
segundo 0 de cada minuto) tem um problema classico: um parceiro pode disparar toda a
cota no ultimo segundo de um minuto e toda a cota de novo no primeiro segundo do
seguinte, dobrando na pratica o limite combinado. A janela deslizante evita isso: cada
requisicao vira uma entrada num sorted set do Redis com o timestamp em milissegundos
como score, um script Lua remove as entradas fora da janela e conta quantas sobraram,
tudo atomicamente (sem essa atomicidade, duas instancias do gateway decidindo ao mesmo
tempo se ainda ha cota criariam uma condicao de corrida). Se a cota estourou, o gateway
responde 429 com o cabecalho `Retry-After` calculado a partir da entrada mais antiga
ainda dentro da janela.

**Auditoria assincrona.** Cada requisicao proxied gera um registro de auditoria (parceiro,
rota, metodo, status, latencia) gravado no Oracle. Essa gravacao acontece numa goroutine
separada da resposta ao parceiro: a auditoria importa, mas nao pode ser o motivo de uma
chamada ao upstream ficar mais lenta.

**Metricas ao vivo via WebSocket nativo.** O painel Nuxt precisa mostrar requisicoes por
segundo, taxa de erro e ranking de consumidores em tempo real. Em vez de o frontend ficar
fazendo polling, o gateway mantem um agregador em memoria que zera a cada janela e empurra
um snapshot por segundo para todo cliente conectado em `/ws`, usando gorilla/websocket
puro (sem framework de realtime por cima, o volume de mensagens nao justifica).

**Sem politica, sem acesso.** Um parceiro sem nenhuma politica de rate limit cadastrada
para uma rota (nem uma especifica, nem a padrao) recebe 403, nao um limite generico. O
acesso a cada rota precisa ser concedido explicitamente, o que reflete a realidade de um
gateway de parceria comercial: ninguem deveria conseguir chamar um sistema legado so
porque tem uma api key valida, precisa ter sido liberado para aquela integracao especifica.

## Stack

- **Backend:** Go, `net/http` puro para roteamento, `net/http/httputil` para o proxy
  reverso, gorilla/websocket para o canal de metricas ao vivo.
- **Banco relacional:** Oracle (config de parceiros, rotas, politicas de rate limit e
  log de auditoria), acessado via `sijms/go-ora`, driver puro em Go que dispensa o
  Oracle Instant Client instalado na imagem.
- **Cache/contadores:** Redis, usado exclusivamente para o rate limiting (sorted sets +
  script Lua).
- **Frontend:** Nuxt, rodando como SPA estatica servida por nginx, consumindo o
  WebSocket de metricas e a API administrativa.
- **Observabilidade:** metricas Prometheus nativas (`client_golang`), logs estruturados
  em JSON com Zap, dashboard Grafana provisionado automaticamente.
- **Testes:** `testing` + testify para os unitarios, miniredis para simular o Redis nos
  testes do rate limiter, `httptest` para simular upstreams nos testes do proxy.

## Modelo de dominio

| Entidade | O que representa |
|---|---|
| Parceiro | Empresa ou app autorizado a consumir o gateway, identificado por uma api key |
| Rota | Prefixo de path publico mapeado para a URL de um servico upstream |
| Politica de rate limit | Requisicoes por minuto permitidas para um parceiro numa rota (ou como padrao do parceiro, quando `route_id` e zero) |
| Log de auditoria | Registro de cada requisicao: parceiro, rota, metodo, status, latencia e horario |

Fluxo de uma requisicao de parceiro:

1. O gateway casa o path recebido com a rota mais especifica cadastrada.
2. Autentica pelo cabecalho `X-API-Key`.
3. Busca a politica de rate limit efetiva (especifica da rota, ou a padrao do parceiro).
4. Verifica a cota na janela deslizante do Redis. Sem cota, responde 429.
5. Encaminha a requisicao para o upstream configurado.
6. Grava o log de auditoria e atualiza as metricas (Prometheus, agregador em memoria e
   Oracle) em paralelo, sem bloquear a resposta ao parceiro.

## Como rodar

Pre requisito: Docker e Docker Compose.

```bash
docker compose up --build
```

Isso sobe, nesta ordem de dependencia: Oracle (com o schema aplicado automaticamente a
partir de `migrations/001_init.sql`), Redis, os dois servicos upstream de exemplo
(`mock-billing` e `mock-inventory`, que simulam sistemas legados), o gateway, o painel
Nuxt, Prometheus e Grafana.

O banco sobe limpo, sem nenhum parceiro ou rota cadastrada. Para ter dados de teste
prontos (parceiros de exemplo, rotas apontando para os mocks e politicas de rate limit),
rode o comando de seed separadamente, depois que o Oracle estiver de pe:

```bash
docker compose run --rm seed
```

O seed imprime no terminal as api keys geradas para cada parceiro de exemplo, use uma
delas no cabecalho `X-API-Key` para testar o proxy:

```bash
curl -H "X-API-Key: cpk_demo_aurora_fintech_0001" http://localhost:8080/billing/invoices
```

### Enderecos depois do `docker compose up`

| Servico | Endereco |
|---|---|
| Gateway (proxy e API administrativa) | http://localhost:8080 |
| Painel Nuxt | http://localhost:3002 |
| Prometheus | http://localhost:9091 |
| Grafana (usuario `admin`, senha `admin`) | http://localhost:3001 |
| Oracle | localhost:1521, servico `FREEPDB1` |

### Endpoints administrativos

Todos exigem `Authorization: Bearer <ADMIN_TOKEN>` (o valor padrao no compose e
`admin-secret-token-troque-isto`, troque em qualquer ambiente real).

```
POST   /admin/partners
GET    /admin/partners?page=1&page_size=20
GET    /admin/partners/{id}
PUT    /admin/partners/{id}
DELETE /admin/partners/{id}

POST   /admin/routes
GET    /admin/routes?page=1&page_size=20
GET    /admin/routes/{id}
PUT    /admin/routes/{id}
DELETE /admin/routes/{id}

POST   /admin/policies
GET    /admin/policies?page=1&page_size=20
DELETE /admin/policies/{id}

GET    /admin/audit-logs?page=1&page_size=20
```

### Rodando sem Docker

Para desenvolvimento rapido do backend sem subir Oracle, defina `USE_MEMORY_STORE=true`
(ver `.env.example`): o gateway sobe com um repositorio em memoria, sem persistencia.
Redis ainda precisa estar disponivel para o rate limiting funcionar.

```bash
go run ./cmd/gateway
```

O frontend roda separado com `npm run dev` dentro de `frontend/`, apontando
`NUXT_PUBLIC_API_BASE` e `NUXT_PUBLIC_WS_BASE` para o gateway local.

## Testes

```bash
go test ./... -race
```

Cobre os unitarios de dominio e agregador de metricas, os testes do rate limiter contra
um Redis simulado (miniredis, incluindo o caso de janela expirando e liberando cota de
novo) e os testes de proxy e do handler de gateway completo (autenticacao, rate limit,
proxy e auditoria) usando `httptest` para simular tanto o parceiro quanto o upstream.

## Observabilidade

O gateway expoe metricas Prometheus em `/metrics`: total de requisicoes por parceiro,
rota e status, duracao das requisicoes, rejeicoes de rate limit, falhas de autenticacao,
erros de upstream e clientes conectados no websocket. O Grafana ja sobe com o dashboard
`Comporta - Gateway` provisionado (pasta `observability/grafana`), sem precisar importar
nada manualmente. Logs sao estruturados em JSON via Zap, prontos para qualquer coletor
de log agregar.

## Estrutura

```
cmd/gateway        binario principal do gateway
cmd/seed           comando separado de seed de dados de demonstracao
internal/domain    entidades e regras de dominio, sem dependencia de infraestrutura
internal/store     repositorios (Oracle e uma implementacao em memoria para testes)
internal/proxy     tabela de rotas e motor de reverse proxy
internal/ratelimit rate limiter de janela deslizante sobre Redis
internal/api       handlers HTTP (administrativos e o proxy autenticado)
internal/wshub     hub de websocket para as metricas ao vivo
internal/stats     agregador em memoria de requisicoes por segundo e taxa de erro
internal/metrics   metricas Prometheus
internal/logging   configuracao do logger Zap
migrations         schema Oracle aplicado automaticamente na primeira subida
upstreams          servicos de exemplo simulando sistemas legados
observability      configuracao do Prometheus e provisionamento do Grafana
frontend           painel Nuxt
```
