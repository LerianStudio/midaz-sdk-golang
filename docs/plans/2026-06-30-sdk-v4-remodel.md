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
| 1 | Specs upstream corretas + núcleo gerado compilando + Client de 2 planos lista `organizations` end-to-end com erro/paginação normalizados | ~~1.1~~ (Plano A), 1.2, 1.3, 1.4 | Detailed (1.1 superseded; codegen de-riscado — Option A provada) |
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

### Epic 1.2: Pipeline de codegen

**Goal:** `oapi-codegen` gera tipos + client de baixo nível, um pacote por plano, com output commitado e reprodutível via `make`.
**Scope:** `internal/` (novos pacotes gerados), `Makefile`, arquivo de config do gerador, `go generate`.
**Dependencies:** Epic 1.1 (specs confiáveis)
**Done when:** `make generate` regenera `internal/genledger/` e `internal/gentracer/` a partir das specs da branch de consolidação; `go build ./...` compila o output; a versão do gerador está pinada.
**Target:** midaz-sdk-golang
**Status:** Pending

#### Task 1.2.1: Configurar oapi-codegen por plano

- [ ] Done

**Context:** O SDK hoje é 100% escrito à mão (`entities/*`, `models/*`) e sofre drift — daí os `*_contract_regression_test.go`. O servidor agora emite **dumps Huma OAS 3.1 nativos**: `components/ledger/api/openapi.huma.yaml` + `components/tracer/api/openapi.huma.yaml` (+ joined `postman/specs/midaz.openapi.{json,yaml}`). Já copiados p/ `api/ledger.openapi.yaml` + `api/tracer.openapi.yaml` (input versionado — SDK não depende do worktree efêmero). Não há gerador configurado hoje.

**Implementation vision (SPIKE-VALIDADO 2026-07-01 — Option A provada, 20.138 LOC compiláveis/plano):**
- **oapi-codegen v2.4.1 NÃO suporta OAS 3.1 nativo** (issue #373: `unhandled Schema type &[array null]`). Solução: **downgrade determinístico 3.1→3.0.3 no próprio SDK** (mantém Plano A fechado; a spec 3.1 continua o contrato canônico, o downgrade é concern de codegen do SDK). Transform Go-nativo (sem Python/Node no toolchain do SDK), 2 regras: (a) `type: [X,"null"]` → `type: X` + `nullable: true` (74 ocorrências: 58 ledger + 16 tracer); (b) `del format` onde `type` ∉ {string,number,integer} (1 `format: boolean` bogus). Flipar `openapi: 3.0.3`. **Nenhum outro 3.1-ism** (zero anyOf/oneOf/const/prefixItems/$defs — schemas já 3.0-shaped). *(Override lane p/ Fred: alternativa = Huma `api.OpenAPI().Downgrade()` server-side no midaz, mais future-proof mas re-abre Plano A.)*
- **Package split types⊥client** (fix sistemático das 5 colisões `*Response`: `AuditEventResponse`/`FeeBillingCalculateResponse`/`HolderAccountResponse`/`ProvisionEncryptionResponse`/`ProvisioningStatusResponse` colidem com os wrappers `{OpID}Response` do client): gerar `models` num pacote (`internal/genledger`) e `client` noutro (`internal/genledgerclient`, importa os types), OU usar naming-config. Decidir na impl conforme a ergonomia; o split é o caminho padrão oapi-codegen.
- Dep `github.com/oapi-codegen/runtime` (v1.4.2). Gerador pinado (`tools.go`/tool directive). `//go:generate` + `make generate` que roda downgrade→codegen. Commitar output gerado (consumidor não gera em build-time). Gerar `client` de baixo nível (não server); a fachada consome esse client.

**Files:**
- Create: `tools.go` (ou `go.mod` tool directive), `api/ledger.openapi.yaml`, `api/tracer.openapi.yaml`, `internal/genledger/gen.go` (config + `//go:generate`), `internal/gentracer/gen.go`
- Modify: `Makefile` (target `generate`)

**Verification:** `make generate && go build ./...` — verde; `git status` mostra output determinístico (rodar duas vezes não muda nada).

**Done when:** Os dois pacotes gerados existem, compilam, e regeneram deterministicamente das specs versionadas.

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

**Context:** `midaz.New` (`midaz.go:224`) hoje semeia OTel + `DefaultConfig`, aplica options, faz `config.Validate()` eager (`midaz.go:270`), e `setupEntity()` busca o token do Access Manager sincronamente na construção (`entity.go:139`). Os 16 serviços dividem um `*HTTPClient` (`service.go:44`) com cache de token + `singleflight` + retry; o Bearer é setado em `http.go:1113`; o 401 dispara refresh via singleflight e replay (`http_retry_response.go:160`).

**Implementation vision:** Manter o padrão de options em duas camadas e a validação eager. Construir dois `http.Client` gerados (um por plano) compartilhando **um** provedor de token (o Access Manager atual, `pkg/auth`). O provider injeta `Authorization: Bearer` em ambos os planos; se `WithTracerAPIKey` foi dado, o client do tracer injeta `X-API-Key` em vez do Bearer. Preservar o refresh via singleflight no 401 e o replay-once. O `Client` expõe dois grupos de serviços (`Ledger.*`, `Tracer.*`) ou mantém promoção plana — decidir na execução conforme a ergonomia do gerado. Reusar `pkg/observability`, `pkg/retry`, `pkg/sdkctx` como estão.

**Files:**
- Modify: `midaz.go`, `entities/entity.go`, `entities/service.go`, `entities/http.go`, `entities/http_retry_response.go`
- Modify/replace: `pkg/auth/access_manager.go` (endpoint de token permanece; provider passa a servir dois planos)

**Verification:** Teste de construção: `midaz.New(WithLedgerURL, WithTracerURL, WithAccessManager)` produz um Client com clients dos dois planos; um teste de 401→refresh→replay contra um server httptest passa.

**Done when:** Client constrói contra os dois planos com token compartilhado e refresh preservado.

### Epic 1.4: Normalização de erro e paginação

**Goal:** Os 3 envelopes de erro do servidor colapsam num `pkg/errors` único; os 3 estilos de paginação colapsam no trinaldo tipado `List/Pages/All`.
**Scope:** `pkg/errors/`, `models/list_opts.go`, `models/cursor_list_opts.go`, `entities/iter.go`, novo adaptador de decodificação.
**Dependencies:** Epic 1.2 (tipos gerados dos envelopes)
**Done when:** Um decodificador mapeia `Error`, `pkg.HTTPError` e `ErrorResponse` para `*errors.Error` com `Retryable()`/`Is*` corretos (incluindo `0177`/`0178`); o trinaldo `List/Pages/All` funciona sobre paginação page-based, cursor-based e offset; unit tests table-driven cobrem os três de cada.
**Target:** midaz-sdk-golang
**Status:** Pending

#### Task 1.4.1: Decodificador de erro unificado

- [ ] Done

**Context:** `pkg/errors` já tem `Error{Category, Code, StatusCode, ...}` com oráculo `Retryable()` e predicados `Is*`, e um adaptador status→categoria isolado. O servidor consolidado emite **três** envelopes: ledger `Error{code,title,message,entityType,fields}`; ledger encryption/protection `pkg.HTTPError{code,title,message,entityType,err}`; tracer `ErrorResponse{code,message,title}`. Não há RFC 9457. Códigos novos a tipar: `0177` (422, tracer denial, **não-retryable**), `0178` (503, tracer unavailable, **retryable**), `0490`/`0491` (skip/requireHolder, 422), `0147`/`0148` (ledger settings).

**Implementation vision:** Se a Epic 1.1 convergir o envelope upstream (esperado), o decodificador simplifica para ~1 shape canônico — ainda assim manter tolerância defensiva a variações. Um decodificador que detecta o envelope pela presença de campos (`fields` vs `err` vs triple plano) e/ou content-type, e mapeia para `*errors.Error`. Preservar `code`/`message`/`title`/`entityType`/`fields`. Estender o mapa de retryabilidade: `0178`→retryable, `0177`→não. Manter o adaptador status→categoria existente como fallback. Não vazar tipos gerados pra fora do SDK — a superfície pública continua `*errors.Error`.

**Files:**
- Create: `entities/error_decoder.go` (ou dentro de `pkg/errors`)
- Modify: `pkg/errors/` (novos códigos + retryabilidade)
- Test: `entities/error_decoder_test.go`

**Verification:** `go test ./pkg/errors/... ./entities/... -run TestErrorDecoder -v` — table-driven cobrindo os três envelopes + `0177`/`0178` retryabilidade.

**Done when:** Os três envelopes decodificam para `*errors.Error` com retryabilidade correta.

#### Task 1.4.2: Normalizador de paginação (trinaldo)

- [ ] Done

**Context:** O SDK hoje distingue page-based (`PageListOpts{Limit,Page}`, `models/list_opts.go:80`) de cursor-based (`CursorListOpts{Limit,Cursor}`, `models/cursor_list_opts.go:36`) no nível de tipo, e expõe `List/Pages/All` com `iter.Seq2` (`flattenPages`, `entities/iter.go:136`). O servidor consolidado é híbrido: a maioria das lists manda `page`+`limit` mas responde `CursorPagination{next_cursor,prev_cursor}`; algumas mandam `cursor`; `packages`/`billing-packages` usam offset `Pagination{total}`; tracer é cursor uniforme.

**Implementation vision:** Manter o split tipado page/cursor na superfície pública (não compila se usar a forma errada), mas o adaptador escolhe a forma real **por endpoint** conforme a spec — não por convenção global. Adicionar um terceiro caso interno para offset (`packages`/`billing`), exposto ao usuário ainda via o trinaldo (a mecânica de offset fica escondida). Preservar `List` (uma página), `Pages` (`iter.Seq2[*ListResponse[T]]`), `All` (`iter.Seq2[T]`), `Collect`/`CollectAll`.

**Files:**
- Modify: `models/list_opts.go`, `models/cursor_list_opts.go`, `entities/iter.go`
- Create: adaptador de offset (interno)
- Test: `entities/iter_test.go` (estender)

**Verification:** `go test ./models/... ./entities/... -run 'TestList|TestIter|TestPagination' -v` — cobre page, cursor e offset alimentando o mesmo trinaldo.

**Done when:** O trinaldo `List/Pages/All` funciona sobre os três estilos de paginação.

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

- **Spec coverage:** Ledger 107 ops cobertas por Phases 2–3 (onboarding 2.1, txn/balances/ops 2.2, routes/rates/counts/metadata 2.3, holders/instruments/composition 3.1, fees/billing 3.2, encryption/protection 3.3). Tracer 31 ops por Phase 4 (rules/limits 4.1, reservations/validations 4.2, audit 4.3). Fundação (config/auth/erro/paginação/codegen) por Phase 1. Drifts upstream por Epic 1.1. Streaming por Phase 6. Sem gap conhecido.
- **Vagueness scan:** Tasks da Phase 1 nomeiam envelopes, códigos (`0177`/`0178`/`0490`/`0147`), paths e comandos concretos. Sem "appropriate"/"TBD" na onda detalhada.
- **Contract consistency:** `pkg/errors.Error` é a superfície pública de erro em 1.4.1 e reusada em 3.3; o trinaldo `List/Pages/All` definido em 1.4.2 é reusado por todas as lists; `LedgerURL`/`TracerURL`/`WithTracerAPIKey` definidos em 1.3.1 e consumidos em 1.3.2 e Phase 4.
- **Phase boundaries:** Cada fase termina em software compilável e testável (Phase 1 lista orgs end-to-end; Phase 2 fecha o money path; etc.).
- **Verification plausibility:** Comandos apontam paths reais do SDK (`pkg/config`, `pkg/errors`, `entities/`, `models/`) e do midaz (`components/tracer/...`).

---

## Apêndice — relatórios de exploração (efêmeros, no scratchpad da sessão)

- `01-server-api-surface.md` — inventário REST completo (ledger + tracer) + delta.
- `02-monorepo-capabilities.md` — componentes e capacidades novas.
- `03-current-sdk-architecture.md` — arquitetura atual (keep/coupled/missing) com anchors file:line.
- `04-grpc-streaming-surface.md` — veredicto REST-only.
- Worktree da branch: `scratchpad/midaz-consolidation`.
