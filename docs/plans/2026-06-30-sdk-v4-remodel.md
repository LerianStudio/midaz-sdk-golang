# SDK v4 Remodel (Midaz Monorepo Consolidation) Implementation Plan

> **For implementers:** Use ring:executing-plans (rolling wave: dispatch each
> wave — a phase or one epic, your choice — as a workflow → review → user
> checkpoint → detail the next phase against the real code → repeat),
> ring:dispatching-workflows to run each phase as a reviewed multi-agent
> workflow (review + contrarian baked in), or ring:running-dev-cycle for the
> full subagent-orchestrated workflow.
> This document is the living source of truth — task elaboration for later
> phases is written back into it during execution.

**Goal:** Remodelar completamente o `midaz-sdk-golang` (`/v4`, in-place breaking) para atender a superfície consolidada do servidor Midaz — dois planos REST (Ledger + Tracer) — via um núcleo gerado da OpenAPI mais uma fachada ergonômica escrita à mão.

**Architecture:** Camadas de baixo pra cima: infra reaproveitada transport-agnostic (`sdkctx`, `observability`, `retry`, `validation`, circuit breaker) → núcleo gerado por `oapi-codegen` (tipos + client de baixo nível, um pacote por plano) → adaptadores finos (3 envelopes de erro → 1 `pkg/errors`; 3 estilos de paginação → 1 trinaldo tipado; HEAD count → `int`; async → poll de balance) → fachada à mão (Client de dois planos com Bearer compartilhado, options, builders fluentes, DSL de transação, helpers de settle). O núcleo regenera quando a spec muda, matando o drift na origem; a fachada carrega o valor ergonômico do SDK.

**Tech Stack:** Go 1.26; `oapi-codegen` (pinado, output commitado); `lib-observability` (OTel); `lib-auth/v2` (token do Access Manager, client-side); `iter.Seq2` para paginação; `testify` + `gomock` para testes.

## Phase Overview

| Phase | Milestone | Epics | Status |
|-------|-----------|-------|--------|
| 1 | Specs upstream corretas + núcleo gerado compilando + Client de 2 planos lista `organizations` end-to-end com erro/paginação normalizados | ~~1.1~~ (Plano A), ~~1.2~~ ✅, 1.3, 1.4 | Detailed (1.1 superseded por Plano A; 1.2 ✅ done — codegen dos 2 planos landed; 1.3/1.4 re-ancorados vs. código gerado) |
| 2 | Caminho do dinheiro completo: onboarding CRUD + ciclo de vida de transação (json/inflow/outflow/annotation + commit/cancel/revert) + balances/operations/routes/asset-rates + counts via HEAD | 2.1, 2.2, 2.3 | Epic-level |
| 3 | Domínios novos do ledger: holders/instruments/composition, fees (packages/estimates), billing, encryption, protection | 3.1, 3.2, 3.3 | Epic-level |
| 4 | Plano Tracer completo: rules (CEL), limits, reservations, validations, audit-events; auth Bearer compartilhado + X-API-Key opcional | 4.1, 4.2, 4.3 | Epic-level |
| 5 | Ergonomia (builders, DSL, `WaitForSettlement`) + docs/exemplos/mapping + testes de contrato regenerados; `make ci` verde | 5.1, 5.2, 5.3 | Epic-level |
| 6 | *(opcional / decisão de produto)* Consumidor de streaming Kafka/CloudEvents | 6.1 | Epic-level |

---

## Context recap (fonte da verdade)

Análise detalhada em `scratchpad/01-server-api-surface.md` (inventário REST completo), `02-monorepo-capabilities.md`, `03-current-sdk-architecture.md`, `04-grpc-streaming-surface.md`. Worktree da branch de consolidação: `scratchpad/midaz-consolidation`.

Decisões travadas com o Fred: (1) breaking in-place no `/v4`, sem shim; (2) híbrido gerado+fachada; (3) normalizar erros e paginação; (4) os dois planos agora. gRPC e streaming são server-internos → SDK **REST-only**.

---

## Phase 1 — Foundation

**Milestone:** `go build ./...` verde; `Client` constrói contra os planos Ledger (Bearer) e Tracer (Bearer compartilhado, X-API-Key opcional); pacotes gerados existem para os dois planos; adaptadores de erro (3→1) e paginação (3→1) unit-testados; um round-trip real (`ListOrganizations`) funciona com erro/paginação normalizados.

### Epic 1.1: Correção de specs upstream no midaz — ✅ SUPERSEDED por Plano A (2026-07-01)

> **SUPERSEDED.** Este épico foi escrito em 2026-06-30 assumindo specs swaggo por-corrigir. **Plano A (Migração Huma) já entregou tudo:** Task 1.1.1 (tracer Bearer+ApiKey por-op) = Plano A Fase 2; Task 1.1.3 (unificar os 3 envelopes → 1 canônico) = Plano A Fases 1+4, envelope único **`Error` RFC 9457** byte-idêntico entre planos, travado por gate (`tests/openapi/error_schema_parity_test.go` + `error_schema_singleton_check` no `make ci`) + juiz LLM PASS. As specs agora são **dumps Huma OAS 3.1 nativos** (`components/{ledger,tracer}/api/openapi.huma.yaml` + joined `postman/specs/midaz.openapi.{json,yaml}`), NÃO os `openapi.yaml` swaggo (deletados). Paths citados abaixo (`openapi.yaml:3836`, `ErrorResponse`, `pkg.HTTPError`, geração swaggo) NÃO existem mais. **Épico reduz a verify-only:** confirmar que os specs Plano A são codegen-ready (feito no spike — ver Epic 1.2). Task 1.1.2 (docs streaming/fees do midaz) fica como follow-up opcional no repo midaz, fora do caminho do SDK.

**Goal:** As specs OpenAPI do ledger e do tracer são **pristine e em paridade entre si** — auth real refletido (Bearer + ApiKey), **envelope de erro unificado num único shape canônico entre os dois apps** (códigos e status preservados), convenções consistentes — e viram fonte confiável de codegen.
**Scope:** Repo `midaz`, branch `feat/monorepo-consolidation`; handlers HTTP do tracer + geração de swagger; **tipos de erro dos dois apps** (`Error`, `pkg.HTTPError`, `ErrorResponse`) e seus mapeamentos código/status; docs (`llms.txt`, `STRUCTURE.md`, `SCOPING.md`).
**Dependencies:** none
**Done when:** `components/tracer/api/openapi.yaml` declara `BearerAuth` **e** `ApiKeyAuth` e cada operação referencia ambos (exceto `/validations` quando forçada a API-key); **os três envelopes de erro (ledger `Error`, `pkg.HTTPError`, tracer `ErrorResponse`) convergem para um único shape canônico, com TODOS os códigos e status preservados** (money-path é terceiro rail — muda-se shape, nunca semântica); docs de streaming dizem Kafka/CloudEvents; docs de fees batem com os códigos numéricos; specs regeneradas sem erro. Done quando uma LLM Opus atue como juiz e declare ambas as specs 'pristine' em qualidade **e em paridade entre si** (assumindo o que o swaggo consegue nos entregar).
**Target:** midaz
**Status:** Done (superseded por Plano A — Fases 1/2/4; specs Huma OAS 3.1 pristine + envelope `Error` unificado + gate PASS + juiz LLM PASS)

#### Task 1.1.1: Declarar Bearer + ApiKey nas specs do tracer

- [ ] Done

**Context:** O middleware real do tracer (`components/tracer/internal/adapters/http/in/middleware/auth_guard.go`) é `Plugin Auth (Bearer JWT lib-auth/v2) > API Key` — `Protect()` usa `authClient.Authorize` quando `PluginAuthEnabled`, senão cai em `apiKeyAuth`. Mas as anotações swagger dos handlers (`rule_handler.go`, `limit_handler.go`, `audit_event_handler.go`, `transaction_validation_handler.go`, reservations) só têm `//	@Security		ApiKeyAuth`, e `components/tracer/api/openapi.yaml:3836` só define o scheme `ApiKeyAuth`. Gerar o SDK dessa spec produziria um client cego pro Bearer — por isso esta correção é pré-requisito do codegen.

**Implementation vision:** Adicionar o scheme `BearerAuth` (apiKey em header `Authorization`, mesmo shape do ledger) ao doc de geração swagger do tracer. Anotar cada handler com ambos os schemes (`@Security BearerAuth` + `@Security ApiKeyAuth`), representando "aceita qualquer um". A rota `/v1/validations` mantém ApiKey-only quando `APIKeyOnlyValidation`/`forceAPIKeyAuth` — anotar essa como só `ApiKeyAuth`. **Não alterar comportamento de runtime**; é correção de documentação/spec apenas. Regenerar `openapi.yaml` + `swagger.json`/`swagger.yaml` + `postman/specs/tracer/*` e `postman/specs/midaz.openapi.*`.

**Files:**
- Modify: `components/tracer/internal/adapters/http/in/rule_handler.go`, `limit_handler.go`, `audit_event_handler.go`, `transaction_validation_handler.go`, e demais handlers com `@Security ApiKeyAuth`
- Modify: doc/config de geração swagger do tracer (`components/tracer/internal/adapters/http/in/swagger.go` e o `@securityDefinitions` do doc raiz)
- Regenerate: `components/tracer/api/openapi.yaml`, `swagger.json`, `swagger.yaml`, `postman/specs/tracer/*`, `postman/specs/midaz.openapi.*`

**Verification:** `grep -c BearerAuth components/tracer/api/openapi.yaml` > 0 e cada op (fora `/validations`) referencia os dois schemes; `make` de geração de swagger do tracer roda sem erro.

**Done when:** A spec do tracer declara `BearerAuth` + `ApiKeyAuth` e as ops referenciam ambos conforme a regra acima.

#### Task 1.1.2: Corrigir docs de streaming e fees

- [ ] Done

**Context:** Divergências doc-vs-realidade confirmadas pelos e2e: `llms.txt`/`STRUCTURE.md` dizem RabbitMQ para streaming, mas a superfície nova é Kafka/CloudEvents (franz-go, `pkg/streaming/events`) — RabbitMQ só carrega o path legacy de async. `SCOPING.md` diz erros de fee `FEE-xxxx`, mas o código usa códigos numéricos (0188/0205/0208/0233).

**Implementation vision:** Atualizar `llms.txt`, `llms-full.txt`, `STRUCTURE.md` para descrever streaming como Kafka/CloudEvents 1.0 (binary mode), reservando RabbitMQ ao path async legacy. Alinhar `SCOPING.md` aos códigos numéricos, cruzando com `pkg/constant/errors.go`. Sem mudança de código de runtime.

**Files:**
- Modify: `llms.txt`, `llms-full.txt`, `STRUCTURE.md`, `SCOPING.md`
- Read (cross-check): `pkg/constant/errors.go`, `pkg/streaming/events/`

**Verification:** `grep -in rabbitmq llms.txt STRUCTURE.md` não retorna referência à superfície de streaming nova; códigos de fee em `SCOPING.md` batem com `pkg/constant/errors.go`.

**Done when:** Docs de streaming e fees refletem a realidade do código.

#### Task 1.1.3: Unificar o envelope de erro entre ledger e tracer *(IMPORTANTÍSSIMO — money path)*

- [ ] Done

**Context:** O servidor consolidado emite **três** envelopes de erro: ledger `Error{code,title,message,entityType,fields}` (mais rico, o mais usado); ledger encryption/protection `pkg.HTTPError{code,title,message,entityType,err}` (adiciona `err` objeto); tracer `ErrorResponse{code,message,title}` (triple plano, subset). Sem RFC 9457. Como a branch de consolidação é **pré-release** (sem consumidor externo travado), este é o momento barato de convergir para um envelope único — e isso é pré-requisito de paridade das specs (o ponto que o Fred marcou como IMPORTANTÍSSIMO). Terceiro rail: money path — muda-se o **shape**, nunca códigos ou semântica.

**Implementation vision:** Convergir para um shape canônico único nos dois apps. **Proposta de trabalho:** `Error{code, title, message, entityType, fields}` (o shape do ledger) como canônico; `ErrorResponse` do tracer ganha `entityType` + `fields` (opcionais) e passa a bater campo-a-campo; `pkg.HTTPError` dropa `err` (ou mapeia seu conteúdo para `fields`). **Preservar TODOS os códigos e mapeamentos de status** — diff de códigos de erro deve ser vazio; é renomeação/uniformização de envelope, não de taxonomia. O shape canônico final é decisão a **confirmar com o Fred no checkpoint** antes de tocar o código de erro do money-path (o LLM-juiz + review do workflow também o escrutinam). Regenerar as specs dos dois apps a partir dos tipos unificados.

**Files:**
- Modify: tipos de erro dos dois apps no repo `midaz` (localizar via `pkg/` e `components/{ledger,tracer}/...`), handlers que os emitem
- Regenerate: `components/{ledger,tracer}/api/openapi.yaml` + swagger + postman specs

**Verification:** LLM-juiz (Opus) declara os dois envelopes em paridade; `grep` confirma um único shape nas duas specs; **diff dos códigos de erro entre antes/depois é vazio** (nenhum código adicionado/removido/renomeado).

**Done when:** Ledger e tracer emitem e declaram um envelope de erro idêntico em shape, com toda a taxonomia de códigos/status preservada.

### Epic 1.2: Pipeline de codegen — ✅ DONE (2026-07-01)

> **DONE.** Landed em 4 commits assinados na branch `feat/midaz-monorepo-consolidation`: `b93e560` (downgrade tool), `862977c` (codegen dos 2 planos), `72144a4` (drift gate), `612e331` (fix base64→byte, follow-up de gate do supervisor). Gate do supervisor (ring:dispatching-workflows) PASS empírico: `go build ./...`+`go vet` verdes nos 2 planos, `make generate` reproduz o output commitado byte-a-byte, determinístico run-to-run, 0 colisões de tipo, specs de input intocadas desde `cbcf559`, sem scope creep em `entities/`/`pkg/config`/`pkg/errors`. **Desvios do vision de 2026-06-30 (rolling-wave — código real ≠ plano):** ver notas abaixo.

**Goal:** `oapi-codegen` gera tipos + client de baixo nível, um pacote por plano, com output commitado e reprodutível via `make`.
**Scope:** `internal/genledger/`, `internal/gentracer/` (gerados), `internal/cmd/specdowngrade/` (tool), `scripts/generate-clients.sh` + `scripts/check-codegen-drift.sh`, `Makefile`, `go.mod` (tool directive).
**Dependencies:** Epic 1.1 (specs confiáveis) — satisfeito por Plano A.
**Done when:** `make generate` regenera `internal/genledger/` e `internal/gentracer/` a partir de `api/{ledger,tracer}.openapi.yaml`; `go build ./...` compila o output; a versão do gerador está pinada; a drift gate (`make check-codegen-drift`, em `verify-sdk`) prova que o output commitado reproduz das specs.
**Target:** midaz-sdk-golang
**Status:** Done

**Desvios do vision original (o que realmente landou):**
1. **Downgrade tool = TRÊS transforms, não dois.** Além de (a) `type:[X,"null"]`→`type:X`+`nullable:true` e (b) strip de `format` bogus, o gate do supervisor adicionou (c) `contentEncoding: base64` (string) → `format: byte`, senão o `estimateFeeCalculation` 200 vinha como `*string` base64 cru em vez de `*[]byte` auto-decodado (footgun num endpoint de fee). Keys 3.1 restantes (`examples` plural, `contentMediaType`) ficam como passthrough tolerado (kin-openapi ignora; não afeta o Go gerado) — documentado no doc-comment do tool.
2. **UM pacote por plano, NÃO o split types⊥client.** As 5 colisões `*Response` foram resolvidas com a flag `response-type-suffix=Resp` (wrappers do client viram `{OpID}Resp`), mantendo `genledger`/`gentracer` num pacote só cada. Mais limpo que o split de 2 pacotes previsto.
3. **`ClientWithResponses` (typed) é a superfície que a fachada vai consumir** (não o `Client` de baixo nível cru). Auth injetada via `WithRequestEditorFn` (ver Epic 1.3.2).
4. **Drift gate (`scripts/check-codegen-drift.sh` + target `check-codegen-drift` em `verify-sdk`)** — análogo SDK-side da check-docs do Plano A: regenera e exige `git diff` zero nos 2 `.gen.go`, provada 3 formas (clean→0, output stale→1, tree pré-sujo→2). `make generate` roda `scripts/generate-clients.sh` (downgrade→oapi-codegen por plano), gerador pinado via `go.mod` tool directive, dep `github.com/oapi-codegen/runtime` v1.4.2.

*(Tasks 1.2.1–1.2.3 fechadas nos commits acima; um `smoke_test.go` em `internal/genledger` assere o client + `ListOrganizations` + o split de colisão. Findings Low remanescentes — drift gate hardcoda os 2 paths `.gen.go`; tool directive marcado `// indirect` — anotados como follow-ups não-bloqueantes.)*

### Epic 1.3: Config de 2 planos + construção do Client

**Goal:** O `Client` constrói contra os planos Ledger e Tracer com um token Bearer compartilhado e X-API-Key opcional pro tracer.
**Scope:** `pkg/config/`, `midaz.go`, `midaz_options.go`, `entities/entity.go`, `.env*.example`.
**Dependencies:** Epic 1.2 (pacotes gerados a consumir)
**Done when:** `midaz.New(...)` constrói um Client de dois planos; token do Access Manager compartilhado; refresh via singleflight; `MIDAZ_CRM_URL` removido, `MIDAZ_TRACER_URL` + X-API-Key opcional adicionados; validação eager preservada.
**Target:** midaz-sdk-golang
**Status:** Pending

#### Task 1.3.1: Reescrever pkg/config para dois planos

- [ ] Done

**Context:** `pkg/config` hoje modela **três** URLs num par de planos: `WithLedgerURL` (`config.go:238`) seta `onboarding`+`transaction` juntos; `crm` default cai na URL do ledger; `/v1` é hardcoded em `entity.go:443`. `MIDAZ_BASE_URL` fan-out pra ambos. Auth é escolha obrigatória (exatamente um de `WithAccessManager`/`WithAnonymous`, `config.go:1206`). O servidor consolidado tem só dois hosts: ledger `:3002` e tracer `:4020`, ambos `/v1`, e removeu os headers de escopo.

**Implementation vision:** Substituir o mapa `ServiceURLs` de 3 chaves por dois campos explícitos: `LedgerURL` e `TracerURL`. Env vars: `MIDAZ_LEDGER_URL`, `MIDAZ_TRACER_URL`; **remover** `MIDAZ_CRM_URL` e o fan-out de `MIDAZ_BASE_URL` (ou redefinir `MIDAZ_BASE_URL` como default de ambos os planos, decisão a documentar). Adicionar `WithTracerAPIKey(string)` como option opcional — quando ausente, o tracer usa o mesmo Bearer do ledger; quando presente, envia `X-API-Key`. Manter `/v1` como prefixo por plano, mas configurável. Manter validação eager (`config.Validate()`). Atualizar os três `.env*.example` e `FromEnvironment` em sincronia (regra do CLAUDE.md do projeto).

**Files:**
- Modify: `pkg/config/config.go` (modelo de URLs, options, `FromEnvironment`), `.env.example`, `.env.local.example`, `.env.production.example`
- Modify: `midaz_options.go` (novas `With*`), `types.go` (constantes de Environment se afetadas)

**Verification:** `go test ./pkg/config/... -run TestFromEnvironment -v` cobrindo os dois planos + X-API-Key opcional; `go build ./...` verde.

**Done when:** Config expõe LedgerURL + TracerURL + WithTracerAPIKey; `MIDAZ_CRM_URL` sumiu; `.env*.example` e `FromEnvironment` batem.

#### Task 1.3.2: Construir o Client de dois planos sobre os pacotes gerados

- [ ] Done

**Context:** `midaz.New` (`midaz.go:224`) hoje semeia OTel + `DefaultConfig`, aplica options, faz `config.Validate()` eager (`midaz.go:270`), e `setupEntity()` busca o token do Access Manager sincronamente na construção (`entity.go:139`). Os 16 serviços dividem um `*HTTPClient` (`service.go:44`) com cache de token + `singleflight` + retry; o 401 dispara refresh via `singleflight.Group tokenRefreshGroup` (`http.go:126`) e replay-once (`http_retry_response.go:205`, `refreshedAuth` latch). **Superfície gerada real (Epic 1.2 landed):** cada plano expõe `genledger.NewClientWithResponses(server string, opts ...ClientOption)` / `gentracer.NewClientWithResponses(...)` → `*ClientWithResponses` com métodos typed `{OpID}WithResponse(ctx, params, reqEditors...)`. Options do gerador: `WithHTTPClient(HttpRequestDoer)` e `WithRequestEditorFn(RequestEditorFn)` onde `RequestEditorFn func(ctx, *http.Request) error`.

**Implementation vision:** Manter o padrão de options em duas camadas e a validação eager. Construir dois `*ClientWithResponses` (um por plano) sobre `internal/genledger` e `internal/gentracer`. **Auth via `RequestEditorFn` compartilhada:** um editor que puxa o token do provider único (`pkg/auth`, Access Manager atual) e injeta `Authorization: Bearer <tok>`; o client do tracer usa um editor alternativo que injeta `X-API-Key` quando `WithTracerAPIKey` foi dado, senão cai no mesmo Bearer. **401→refresh→replay-once:** a lógica hoje mora no wrapper `entities/HTTPClient` (acima do `http.Client`); como o gerado só aceita um `HttpRequestDoer` via `WithHTTPClient`, migrar essa lógica (incluindo o `singleflight` de refresh e o latch `refreshedAuth`) para um `http.RoundTripper` que embrulha o `http.Transport` pooled, e passar `WithHTTPClient(&http.Client{Transport: authRefreshRoundTripper})`. Assim o gerado só emite requests; o RoundTripper concentra Bearer/X-API-Key + refresh + replay. Reusar `pkg/observability`, `pkg/retry`, `pkg/sdkctx` como estão (o retry de transporte pode compor no mesmo RoundTripper ou num wrapper externo — decidir na execução). O `Client` expõe dois grupos (`Ledger.*`, `Tracer.*`) ou promoção plana — decidir conforme a ergonomia da fachada.

**Files:**
- Modify: `midaz.go`, `entities/entity.go`, `entities/service.go`, `entities/http.go`, `entities/http_retry_response.go`
- Create: um `http.RoundTripper` de auth+refresh (provável `entities/auth_roundtripper.go`) que substitui o 401-handling do wrapper `HTTPClient`
- Modify: `pkg/auth/access_manager.go` (endpoint de token permanece; provider passa a servir os dois planos via a RequestEditorFn/RoundTripper)

**Verification:** Teste de construção: `midaz.New(WithLedgerURL, WithTracerURL, WithAccessManager)` produz um Client com `*ClientWithResponses` dos dois planos; um teste de 401→refresh→replay contra um `httptest.Server` passa (o RoundTripper reautentica e reexecuta uma vez); um teste confirma `X-API-Key` no tracer quando `WithTracerAPIKey` é dado e Bearer caso contrário.

**Done when:** Client constrói os dois `*ClientWithResponses` com token Bearer compartilhado (X-API-Key opcional no tracer) e o refresh-once no 401 preservado via RoundTripper.

### Epic 1.4: Normalização de erro e paginação

**Goal:** O envelope de erro único RFC 9457 do servidor (unificado por Plano A) mapeia limpo pra `pkg/errors`; os estilos de paginação do servidor (page-request/cursor-response + offset) colapsam no trinaldo tipado `List/Pages/All`.
**Scope:** `pkg/errors/`, `models/list_opts.go`, `models/cursor_list_opts.go`, `entities/iter.go`, novo adaptador de decodificação.
**Dependencies:** Epic 1.2 (tipos gerados: `genledger.Error`/`gentracer.Error` + `Pagination`)
**Done when:** Um decodificador mapeia o `Error` RFC 9457 gerado (idêntico nos 2 planos) para `*errors.Error` com `Retryable()`/`Is*` corretos (ex.: 503→retryable, 422→não); o trinaldo `List/Pages/All` funciona sobre a paginação page-request/cursor-response e sobre offset; unit tests table-driven cobrem cada caso.
**Target:** midaz-sdk-golang
**Status:** Pending

#### Task 1.4.1: Decodificador de erro unificado (envelope RFC 9457 único)

- [ ] Done

**Context:** `pkg/errors` já tem `Error{Category, Code, StatusCode, ...}` com oráculo `Retryable()` e predicados `Is*`, e um adaptador status→categoria isolado. **Premissa mudou (Plano A):** o servidor NÃO emite mais três envelopes divergentes. Os dois planos agora emitem **um** envelope RFC 9457 idêntico — o tipo gerado `Error{Code, Detail, Errors, Instance, Status, Title, Type}` (`internal/genledger/ledger.gen.go:261` ≡ `internal/gentracer/tracer.gen.go:60`, byte-idêntico), com `ErrorDetail{Location, Message, Value}` para erros de campo. `Code` agora é `<SERVICE>-NNNN` (com prefixo de serviço). Os `pkg.HTTPError` e `ErrorResponse` sumiram — a sniffagem por presença de campo virou desnecessária.

**Implementation vision:** O decodificador vira um mapa de **um shape só** → `*errors.Error`: `Code`→código, `Detail`→mensagem, `Title`→title, `Status`→StatusCode, `Errors []ErrorDetail`→o field-errors do SDK (`Location`+`Message`+`Value` por campo). Retryabilidade keya primeiro no `Status` (503→retryable, 5xx idem; 422/4xx→não), com override por `Code` para os casos que o status não captura (o antigo `0178`/503 unavailable→retryable, `0177`/422 denial→não — agora sob o `Code` prefixado; casar por sufixo numérico p/ ser robusto ao prefixo). Manter o adaptador status→categoria existente como base. **Não vazar tipos gerados** pra fora do SDK — a superfície pública continua `*errors.Error`; o decoder aceita o `Error` gerado (ou os bytes crus do `application/problem+json`) e devolve `*errors.Error`. Tolerância defensiva a campos ausentes (todos os campos do envelope são ponteiros/opcionais).

**Files:**
- Create: `entities/error_decoder.go` (ou dentro de `pkg/errors`)
- Modify: `pkg/errors/` (mapa de retryabilidade por status + códigos prefixados)
- Test: `entities/error_decoder_test.go`

**Verification:** `go test ./pkg/errors/... ./entities/... -run TestErrorDecoder -v` — table-driven cobrindo o envelope RFC 9457 (com e sem `Errors[]`), retryabilidade por `Status` (503 vs 422) e o override por `Code` (sufixos `0177`/`0178`).

**Done when:** O envelope RFC 9457 único decodifica para `*errors.Error` com retryabilidade correta por status e por código, e os `ErrorDetail` viram field-errors.

#### Task 1.4.2: Normalizador de paginação (trinaldo)

- [ ] Done

**Context:** O SDK hoje distingue page-based (`PageListOpts{Limit,Page}`, `models/list_opts.go:80`) de cursor-based (`CursorListOpts{Limit,Cursor}`, `models/cursor_list_opts.go:36`) no nível de tipo, e expõe `List/Pages/All` com `iter.Seq2` (`flattenPages`, `entities/iter.go:136`). **Superfície gerada real (Epic 1.2):** os params de list são query strings — ex. `ListOrganizationsParams{Limit *string, Page *string, SortOrder *string}` (`ledger.gen.go:756`, note `*string`, não `*int`); a resposta carrega `Pagination{Limit, NextCursor *string, PrevCursor *string, ...}` (`ledger.gen.go:621`). Confirma o híbrido previsto: **request page+limit, response cursor**. `packages`/`billing-packages` usam offset; tracer é cursor uniforme.

**Implementation vision:** Manter o split tipado page/cursor na superfície pública (não compila se usar a forma errada), mas o adaptador escolhe a forma real **por endpoint** conforme a spec — não por convenção global. Como os params gerados são `*string`, o adaptador serializa `Limit`/`Page`/`Cursor` (int→string) na borda e lê `NextCursor`/`PrevCursor` da `Pagination` da resposta pra encadear as páginas. Adicionar um terceiro caso interno para offset (`packages`/`billing`), exposto ao usuário ainda via o trinaldo (a mecânica de offset fica escondida). Preservar `List` (uma página), `Pages` (`iter.Seq2[*ListResponse[T]]`), `All` (`iter.Seq2[T]`), `Collect`/`CollectAll`.

**Files:**
- Modify: `models/list_opts.go`, `models/cursor_list_opts.go`, `entities/iter.go`
- Create: adaptador de offset (interno)
- Test: `entities/iter_test.go` (estender)

**Verification:** `go test ./models/... ./entities/... -run 'TestList|TestIter|TestPagination' -v` — cobre page, cursor e offset alimentando o mesmo trinaldo.

**Done when:** O trinaldo `List/Pages/All` funciona sobre os três estilos de paginação.

> **1.4.2 fechado como no-op (2026-07-01):** a premissa da task ficou obsoleta — o trinaldo `List/Pages/All` + o split page/cursor + o encadeamento por-endpoint (`Page++` vs `NextCursor`) **já existem e passam** (`TestTransactionsEntity_ListTransactions_UsesCursorPagination`, List de Portfolios/Segments). O adaptador de offset interno é YAGNI: nenhuma entidade `packages`/`billing` existe ainda (Phase 3). Contrarian lens de paginação confirmou: sem defeito. Sem mudança de produção.

---

### Epic 1.R: Remediação do gate de fechamento da Phase 1 (2026-07-01)

**Goal:** zerar os 9 findings do wave de fechamento (`wxcd3fcvo`, PASS) antes de propagar a fachada-exemplar Organizations para os ~10 recursos da Phase 3. Motivação: o exemplar é copiado N vezes — corrigir o template é O(1); corrigir N cópias depois é O(n).
**Scope:** `.env.local.example`, `.env.production.example`, `pkg/errors/`, `entities/organizations_facade.go`, `entities/auth_roundtripper.go` (+ testes).
**Dependencies:** Epics 1.3+1.4 (landed, commits `810d90d`..`655a636`).
**Status:** Doing

Findings a corrigir (severidade do harness → decisão do supervisor):
1. **[Med → fix]** `MIDAZ_ALLOW_INSECURE_HTTP` (data-plane) lido por `FromEnvironment:930` mas ausente de `.env.local.example`/`.env.production.example` — viola o invariante CLAUDE.md (3 `.env*.example` = lista autoritativa em sincronia). Adicionar `=false` com comentário distinguindo do `MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP` (access-manager-plane).
2. **[Med → fix, money-path]** Fachada descarta `X-Request-ID` do servidor no erro (`organizations_facade.go:56` passa `""`). Propagar `resp.HTTPResponse.Header.Get("X-Request-ID")` → preserva correlação server↔client em falhas 503/409-idempotência. Teste: `Error.RequestID` populado no path de erro.
3. **[Med → fix]** `IncludeDeleted` silenciosamente inerte na fachada: o modelo expõe `OrganizationsFilters.IncludeDeleted` mas `listOrganizationsParams` não o propaga e o param gerado não tem `include_deleted`. Injetar via `genledger.RequestEditorFn` (`include_deleted=true`, igual ao legado `ledgers_list_opts.go:58`). **Flag:** spec OAS do ledger omite `include_deleted` de ListOrganizations — gap server-side a fechar depois (regen nativo).
4. **[Med → fix, money-path]** `errUnrewindableBody` fallback (`auth_roundtripper.go:106`) sem teste — guarda o invariante de replay pós-401. Teste: `Body != nil, GetBody == nil` + 401 → 401 original aflora, sem replay/panic.
5. **[Low → fix, money-path]** Código idempotência `0084` inalcançável no formato prefixado real (`LEDGER-0084`): exact-match falha, cai no suffix map novo que não lista `0084`. Adicionar `0084` → `CodeIdempotency` ao suffix map (`errors.go:~2044`). Pré-existente (`1c60073`), dobrado aqui porque o wave criou o fix site.
6. **[Med → fix]** `listOrganizationsParams` branches de filtro sem teste (68.8%): table-test SortDirection/StartDate/EndDate/LegalName/Status → campo gerado certo.
7. **[Low → fix]** Precedência decoder (envelope Status vs transport status) sem teste: caso onde discordam → envelope Status vence categoria/retryabilidade.
8. **[Low → fix]** `injectAuth` erro-do-provider + `Planes()` nil-safe sem cobertura (0%).
9. **[Low → fix]** Comentário singleflight enganoso (`auth_roundtripper.go:43`): o group é por-roundtripper, não cross-plane; o colapso real é em `GetTokenFromAccessManager`.

**Done when:** os 9 corrigidos com TDD onde há mudança de comportamento; `go build`/`go vet`/`make test` verdes; invariantes money-path intactos (X-Idempotency estável no replay, códigos/status/retryabilidade preservados); arquivos gerados intocados.

---

## Phase 2 — Ledger core (money path)

**Milestone:** Onboarding CRUD completo e o ciclo de vida de transação end-to-end funcionam contra o ledger, com counts via HEAD e skips gated por settings.

### Epic 2.1: Recursos de onboarding

**Goal:** Organizations, ledgers (+settings tri-bloco `accounting`/`overrides`/`tracer`), accounts (+lookup por alias, sub-lists de balances/operations), assets, portfolios, segments, account-types e metadata-indexes funcionam via a fachada.
**Scope:** `entities/`, `models/` (recursos de onboarding), fachada sobre `internal/genledger`.
**Dependencies:** Phase 1
**Done when:** CRUD de cada recurso passa contra um server httptest/mock alimentado pela spec; counts usam HEAD → `X-Total-Count`; ledger settings PATCH tolera o enforcement de chave (0147/0148).
**Target:** midaz-sdk-golang
**Status:** Pending

*(Tasks elaboradas por ring:executing-plans quando a Phase 1 pousar.)*

### Epic 2.2: Ciclo de vida de transação + balances + operations

**Goal:** Os quatro paths de create (`/json`,`/inflow`,`/outflow`,`/annotation`), `commit`/`cancel`/`revert`, balances (split compact `Balance` vs `BalanceHistory`) e operations funcionam; skips (`TransactionSkip{fees,tracer}`) honrados só quando `settings.overrides` habilita, com 422 `0490` claro caso contrário.
**Scope:** `entities/transactions*.go`, `models/transaction*.go`, balances/operations.
**Dependencies:** Epic 2.1
**Done when:** Criar/commitar/cancelar/reverter passa; a resposta expõe fee legs + `amount` inflado/líquido; `feesSkipped`/`tracerSkipped` refletidos; `/dsl` mantido só como atalho para `/json`.
**Target:** midaz-sdk-golang
**Status:** Pending

### Epic 2.3: Routes, asset-rates, counts e metadata-indexes

**Goal:** Operation-routes, transaction-routes, asset-rates (create via `PUT .../asset-rates` com `from`/`to` no body), counts via HEAD e metadata-indexes funcionam.
**Scope:** `entities/`, `models/` dos recursos acima.
**Dependencies:** Epic 2.1
**Done when:** CRUD passa; asset-rates create manda códigos no body; counts parseiam `X-Total-Count`; metadata-indexes lida com array não-paginado.
**Target:** midaz-sdk-golang
**Status:** Pending

---

## Phase 3 — Ledger new domains (CRM + fees + billing + crypto)

**Milestone:** Holders/instruments/composition, fees (packages/estimates), billing e encryption/protection funcionam via a fachada.

### Epic 3.1: Holders, Instruments, Composition

**Goal:** Holders (re-homed no ledger, path-based), instruments (+related-parties), e composition (holder+account+instrument numa chamada, com envelope não-atômico `{account,instrument,instrumentError}`) funcionam.
**Scope:** `entities/`, `models/` (holders/instruments/composition), idempotência via `X-Idempotency-Key`.
**Dependencies:** Phase 2
**Done when:** CRUD de holders/instruments passa; composition expõe `instrumentError` sem engolir (sem rollback do lado do servidor); CRM enforcement (422 `0491` requireHolder) tipado.
**Target:** midaz-sdk-golang
**Status:** Pending

### Epic 3.2: Fees (packages/estimates) + Billing

**Goal:** Packages (definições de fee, offset-paginated), estimates (dry-run), billing-packages e billing-calculate funcionam.
**Scope:** `entities/`, `models/` de fees e billing.
**Dependencies:** Phase 2
**Done when:** CRUD de packages passa (offset pagination via trinaldo); `/estimates` retorna cálculo dry-run; billing-calculate retorna results+summary; códigos numéricos de fee tipados.
**Target:** midaz-sdk-golang
**Status:** Pending

### Epic 3.3: Encryption + Protection

**Goal:** Encryption (`/provision`, `/status`) e protection (`/audit`) funcionam, incluindo a decodificação do envelope `pkg.HTTPError`.
**Scope:** `entities/`, `models/` de encryption/protection.
**Dependencies:** Phase 2, Epic 1.4 (decodificador cobre `pkg.HTTPError`)
**Done when:** Provision/status/audit passam; 404 = legacy mode tratado.
**Target:** midaz-sdk-golang
**Status:** Pending

---

## Phase 4 — Tracer plane

**Milestone:** Os cinco primitivos do tracer funcionam via a fachada, com auth Bearer compartilhado e X-API-Key opcional.

### Epic 4.1: Client do tracer + rules + limits

**Goal:** O plano tracer autentica (Bearer compartilhado ou X-API-Key), e rules (CEL, lifecycle DRAFT→ACTIVE→INACTIVE) e limits (+usage) funcionam.
**Scope:** fachada sobre `internal/gentracer`, `entities/`, `models/` de rules/limits.
**Dependencies:** Phase 1, Epic 1.1 (spec do tracer com Bearer)
**Done when:** CRUD + activate/deactivate/draft de rules e limits passa; `422` de custo de CEL e `limit/usage` tratados.
**Target:** midaz-sdk-golang
**Status:** Pending

### Epic 4.2: Reservations + validations

**Goal:** Reservations (duas fases: reserve → confirm/release por transaction_id ou por id) e validations (avalia txn → ALLOW/DENY/REVIEW, idempotente em `requestId`) funcionam.
**Scope:** `entities/`, `models/` de reservations/validations.
**Dependencies:** Epic 4.1
**Done when:** Reserve/confirm/release passam (incluindo idempotência `flipped=0`); validations retorna decisão + matched rules + limits excedidos; 200 (replay) vs 201 (novo) distinguidos.
**Target:** midaz-sdk-golang
**Status:** Pending

### Epic 4.3: Audit events

**Goal:** Audit-events (list/get/verify da trilha hash-chain SHA-256) funcionam.
**Scope:** `entities/`, `models/` de audit-events.
**Dependencies:** Epic 4.1
**Done when:** List/get passam; `/verify` retorna `HashChainVerificationResult`.
**Target:** midaz-sdk-golang
**Status:** Pending

---

## Phase 5 — Ergonomics, docs, contract tests

**Milestone:** Helpers ergonômicos completos, docs/exemplos atualizados, testes de contrato regenerados; `make ci` verde.

### Epic 5.1: Helpers ergonômicos

**Goal:** Builders fluentes (com `FieldErrors`), DSL de transação e `WaitForSettlement` (poll de balance, não de status) funcionam.
**Scope:** `models/transaction_dsl.go`, builders, novo helper de settle em `pkg/transaction` ou `entities/`.
**Dependencies:** Phases 2–4
**Done when:** `WaitForSettlement` polla balance com backoff/timeout e documenta que 201 ≠ liquidado; DSL aponta `/json`; builders cobrem os recursos novos.
**Target:** midaz-sdk-golang
**Status:** Pending

### Epic 5.2: Docs, exemplos, mapping

**Goal:** `README.md`, `docs/README.md`, `docs/mapping/`, `docs/examples.md` e o `mass-demo-generator` refletem a superfície nova.
**Scope:** `docs/`, `examples/`, `README.md`.
**Dependencies:** Phases 2–4
**Done when:** Exemplos rodam contra o stack novo; mapping público/interno atualizado; `make demo-data` funciona.
**Target:** midaz-sdk-golang
**Status:** Pending

### Epic 5.3: Testes de contrato + cobertura

**Goal:** Os `*_contract_regression_test.go` são regenerados/validados contra as specs corrigidas; cobertura ≥80% na lógica crítica nova.
**Scope:** testes em `entities/`, `models/`.
**Dependencies:** Phases 2–4
**Done when:** `make ci` verde; testes de contrato batem com as specs versionadas.
**Target:** midaz-sdk-golang
**Status:** Pending

---

## Phase 6 — Streaming consumer *(opcional / decisão de produto)*

**Milestone:** Consumidor tipado de eventos Kafka/CloudEvents, se Lerian decidir expor consumo ao usuário do SDK.

### Epic 6.1: Consumidor Kafka/CloudEvents

**Goal:** Helper de consumo de eventos reusando os payloads públicos de `pkg/streaming/events` do midaz.
**Scope:** novo pacote `pkg/streaming` no SDK.
**Dependencies:** decisão de produto (não é requisito da consolidação — emissão é producer-only server-interna hoje)
**Done when:** A definir se/quando priorizado.
**Target:** midaz-sdk-golang
**Status:** Pending

---

## Self-review

- **Spec coverage:** Ledger 107 ops cobertas por Phases 2–3 (onboarding 2.1, txn/balances/ops 2.2, routes/rates/counts/metadata 2.3, holders/instruments/composition 3.1, fees/billing 3.2, encryption/protection 3.3). Tracer 31 ops por Phase 4 (rules/limits 4.1, reservations/validations 4.2, audit 4.3). Fundação: Epic 1.1 (Plano A ✅) + codegen 1.2 (✅) + config/auth 1.3 + erro/paginação 1.4. Streaming por Phase 6. Sem gap conhecido.
- **Vagueness scan:** Tasks da Phase 1 nomeiam o envelope RFC 9457 (`Error`/`ErrorDetail`), códigos (sufixos `0177`/`0178`/`0490`/`0147`), tipos gerados reais (`ClientWithResponses`, `Pagination`, `RequestEditorFn`), paths e comandos concretos. Sem "appropriate"/"TBD" na onda detalhada.
- **Contract consistency:** `pkg/errors.Error` é a superfície pública de erro em 1.4.1 e reusada em 3.3; o único envelope upstream é o RFC 9457 `Error` gerado (idêntico nos 2 planos), consumido só pelo decoder de 1.4.1; o trinaldo `List/Pages/All` definido em 1.4.2 é reusado por todas as lists; `LedgerURL`/`TracerURL`/`WithTracerAPIKey` definidos em 1.3.1 e consumidos em 1.3.2 (via RequestEditorFn/RoundTripper) e Phase 4.
- **Phase boundaries:** Cada fase termina em software compilável e testável (Phase 1 lista orgs end-to-end; Phase 2 fecha o money path; etc.).
- **Verification plausibility:** Comandos apontam paths reais do SDK (`pkg/config`, `pkg/errors`, `entities/`, `models/`) e do midaz (`components/tracer/...`).

---

## Apêndice — relatórios de exploração (efêmeros, no scratchpad da sessão)

- `01-server-api-surface.md` — inventário REST completo (ledger + tracer) + delta.
- `02-monorepo-capabilities.md` — componentes e capacidades novas.
- `03-current-sdk-architecture.md` — arquitetura atual (keep/coupled/missing) com anchors file:line.
- `04-grpc-streaming-surface.md` — veredicto REST-only.
- Worktree da branch: `scratchpad/midaz-consolidation`.
