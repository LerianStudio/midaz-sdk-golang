# SDK v4 Remodel (Midaz Monorepo Consolidation) Implementation Plan

> **For implementers:** Use ring:executing-plans (rolling wave: dispatch each
> wave — a phase or one epic, your choice — as a workflow → review → user
> checkpoint → detail the next phase against the real code → repeat),
> ring:dispatching-workflows to run each phase as a reviewed multi-agent
> workflow (review + contrarian baked in), or ring:running-dev-cycle for the
> full subagent-orchestrated workflow.
> This document is the living source of truth — task elaboration for later
> phases is written back into it during execution.

**Goal:** Remodelar o `midaz-sdk-golang` (`/v4`, breaking in-place) para a superfície consolidada do servidor Midaz — dois planos REST (Ledger + Tracer) — via um núcleo gerado da OpenAPI mais uma fachada ergonômica escrita à mão.

**Architecture:** De baixo pra cima: infra transport-agnostic reaproveitada (`sdkctx`, `observability`, `retry`, `validation`) → núcleo gerado por `oapi-codegen` (tipos + `ClientWithResponses` de baixo nível, um pacote por plano) → adaptadores finos (1 envelope de erro RFC 9457; 1 trinaldo de paginação tipado `List/Pages/All`) → fachada à mão (`entities/*_facade.go`) sobre um Client de dois planos com Bearer compartilhado + X-API-Key opcional no tracer. O núcleo regenera quando a spec muda (drift-gated), matando o drift na origem; a fachada carrega o valor ergonômico e mantém os tipos gerados fora da superfície pública.

**Tech Stack:** Go 1.26; `oapi-codegen` (pinado via `go.mod` tool directive, output commitado); `lib-observability` (OTel); `lib-auth/v2` (token do Access Manager, client-side); `iter.Seq2` para paginação; `testify` + `gomock` + `httptest` para testes.

## Phase Overview

| Phase | Milestone | Epics | Status |
|-------|-----------|-------|--------|
| 1 | Núcleo gerado compila; Client de 2 planos lista `organizations` end-to-end com erro (RFC 9457) e paginação normalizados | 1.1, 1.2, 1.3, 1.4, 1.R | **Complete** |
| 2 | Money path completo: onboarding CRUD + ciclo de transação (json/inflow/outflow/annotation + commit/cancel/revert) + balances/operations/routes/asset-rates + counts | 2.1, 2.2, 2.R, 2.3 | **Complete** (2.1, 2.2, 2.R, 2.3 todos Done) |
| 3 | Domínios novos do ledger: holders/instruments/composition, fees (packages/estimates), billing, encryption/protection | 3.1, 3.2, 3.3 | Epic-level |
| 4 | Plano Tracer completo: rules (CEL), limits, reservations, validations, audit-events | 4.1, 4.2, 4.3 | Epic-level |
| 5 | Ergonomia (builders, DSL, `WaitForSettlement`) + cutover do accessor/deleção do legado + docs/exemplos/mapping; `make ci` verde | 5.1, 5.2, 5.3 | Epic-level |
| 6 | *(opcional / decisão de produto)* Consumidor de streaming Kafka/CloudEvents | 6.1 | Epic-level |

---

## Contratos travados (fonte da verdade — money-path)

Invariantes que atravessam fases. Alterar qualquer um destes é decisão de arquitetura, não de execução.

1. **Decisões do Fred (base):** (a) breaking in-place no `/v4`, sem shim; (b) híbrido gerado+fachada; (c) normalizar erro e paginação; (d) os dois planos agora. gRPC/streaming são server-internos → SDK **REST-only**.
2. **Envelope de erro único (RFC 9457):** os dois planos emitem um `Error{Code, Detail, Errors, Instance, Status, Title, Type}` byte-idêntico (`internal/genledger` ≡ `internal/gentracer`), com `ErrorDetail{Location, Message, Value}`. `Code` é `<SERVICE>-NNNN` (prefixado). Decoder = `pkg/errors.DecodeProblemJSON(status, body, requestID)`; retryabilidade keia no `Status` com override por sufixo de `Code` (`0177`/`0178`). `LEDGER-0084` → `CodeIdempotency` (409, não-retryável, `CategoryConflict`) via suffix map. **A superfície pública de erro é sempre `*errors.Error` — tipos gerados nunca vazam.**
3. **Write-facade pattern (money-path, Phases 2–4):** os bodies de write no gerado são `openapi_types.File` (Plano A migrou writes como corpo opaco). Escrever via `Create{X}WithBodyWithResponse(ctx, ids, params, "application/json", body, authEditors...)` com `body = bytes.NewReader(json.Marshal(models.{X}Input))`. **O body TEM que ser `bytes.NewReader`/`bytes.NewBuffer`** — só tipos concretos fazem `http.NewRequest` popular `GetBody`, o hook que o auth RoundTripper usa pra rebobinar e reexecutar após 401. Body não-rebobinável → replay recusado → write perdido silenciosamente. Helpers package-level firmados no exemplar: `writeJSON[T]`, `decodeOne[T]`, `isSuccess` (2xx), `statusOf`, `requestIDOf`, `setQueryParam`, `strPtr`, `flattenPages` (`entities/organizations_facade.go`). **Sucesso de write onboarding = HTTP 200, não 201** (o gerado só popula `JSON200` em status==200).
4. **Auth 401→refresh→replay-once (`entities/auth_roundtripper.go`):** o RoundTripper injeta Bearer/X-API-Key, no 401 invalida+refaz o token (singleflight) e reexecuta o request idêntico UMA vez via `req.Clone()` + `GetBody`. `X-Idempotency`/`X-TTL` do caller sobrevivem byte-a-byte ao replay (só `Authorization` é reescrito). Body não-rebobinável → `errUnrewindableBody` → aflora o 401 original sem replay.
5. **Idempotência = `X-Idempotency` + `X-TTL`, NUNCA `X-Idempotency-Key`.** O runtime `lib-commons/v5` usa `X-Idempotency`. `X-Idempotency-Key` é um mito de doc-comment (Plano A Fase 3 pegou como CRITICAL). Terceiro rail.
6. **Filtros sem slot no gerado NÃO são dropados:** injetar via `setQueryParam` req-editor (re-lê/Set/re-Encode preservando os params já codificados). Gap de spec server-side conhecido: a OAS do ledger omite `include_deleted`/`holder_id` de várias list-ops → follow-up no midaz (regen gera o campo nativo e o editor manual sai).
7. **Coexistência (até Phase 5):** as fachadas são construídas + e2e-testadas mas NÃO plugadas em `client.X` (que segue no legado). Cutover do accessor + deleção do legado = passo atômico único na Phase 5. Reversível.
8. **Arquivos gerados (`internal/gen*`) são intocáveis à mão.** Só mudam via `make generate` (drift-gated). Contratos entre fachada e gerado: `ClientWithResponses`, `{OpID}WithResponse(...)`/`{OpID}WithBodyWithResponse(...)`, `Pagination{Limit, NextCursor, PrevCursor, Page, ...}`, `RequestEditorFn`.

Exploração-fonte (efêmera, no scratchpad da sessão): `01-server-api-surface.md` (inventário REST ledger+tracer), `02-monorepo-capabilities.md`, `03-current-sdk-architecture.md`, `04-grpc-streaming-surface.md` (veredicto REST-only).

---

## Phase 1 — Foundation ✅ Complete

**Milestone entregue:** `go build ./...` verde nos 2 planos; `Client` constrói contra Ledger (Bearer) e Tracer (Bearer compartilhado, X-API-Key opcional); pacotes gerados existem e regeneram determinísticos; decoder de erro RFC 9457 (3→1) e trinaldo de paginação unit-testados; `ListOrganizations` faz round-trip real com erro/paginação normalizados. Commits `cbcf559`..`4e546fd` (2 waves + remediação). Todos os invariantes acima verificados empiricamente no gate do supervisor.

### Epic 1.1: Specs upstream pristine + paridade — Done (superseded por Plano A)

**O que ficou:** este épico assumia specs swaggo por-corrigir. **Plano A (migração Huma) entregou tudo antes:** tracer com Bearer+ApiKey por-op (Plano A Fase 2); os três envelopes de erro convergidos num `Error` RFC 9457 canônico byte-idêntico entre planos, travado por gate (`tests/openapi/error_schema_parity_test.go` + `error_schema_singleton_check` no `make ci`) + juiz LLM PASS (Plano A Fases 1+4). As specs de codegen são dumps Huma OAS 3.1 nativos (`api/{ledger,tracer}.openapi.yaml`), não os `openapi.yaml` swaggo (deletados). Épico reduziu-se a verify-only: confirmar que os specs Plano A são codegen-ready (feito no spike de 1.2).

**Deviation p/ Phase 3:** docs de streaming/fees do midaz (RabbitMQ→Kafka/CloudEvents; `FEE-xxxx`→códigos numéricos) ficam como follow-up opcional no repo midaz, fora do caminho do SDK.

### Epic 1.2: Pipeline de codegen — Done

**O que landou** (`b93e560`, `862977c`, `72144a4`, `612e331`, `9cc7f8b`): `oapi-codegen` gera `internal/genledger/` + `internal/gentracer/` a partir de `api/{ledger,tracer}.openapi.yaml`; output commitado; `make generate` reproduz byte-a-byte; gerador pinado via `go.mod` tool directive; drift gate (`scripts/check-codegen-drift.sh` em `verify-sdk`) exige `git diff` zero nos `.gen.go`.

**Deviations travadas (código real ≠ vision de 2026-06-30):**
- **Downgrade tool = TRÊS transforms** (`internal/cmd/specdowngrade`): (a) `type:[X,"null"]`→`type:X`+`nullable:true`; (b) strip de `format` bogus; (c) `contentEncoding:base64`→`format:byte` (senão `estimateFeeCalculation` 200 vinha `*string` base64 cru em vez de `*[]byte`).
- **UM pacote por plano**, não split types⊥client: as 5 colisões `*Response` resolvidas com `response-type-suffix=Resp`.
- **`ClientWithResponses` (typed)** é a superfície que a fachada consome; auth via `WithRequestEditorFn`/RoundTripper.

### Epic 1.3: Config de 2 planos + construção do Client — Done

**O que landou** (`810d90d` config, `4a56305` clients+RoundTripper): `pkg/config` modela dois planos explícitos (`LedgerURL`, `TracerURL`), `MIDAZ_CRM_URL` removido, `WithTracerAPIKey` opcional; `midaz.New` constrói dois `*ClientWithResponses` com Bearer compartilhado; o 401→refresh→replay-once migrou do wrapper `HTTPClient` legado para o `authRefreshRoundTripper` (invariante money-path #4 acima). Validação eager preservada.

### Epic 1.4: Normalização de erro e paginação — Done

**O que landou** (`7b6c6da`): decoder do envelope RFC 9457 único → `*errors.Error` com retryabilidade por `Status` + override por sufixo de `Code`; `ErrorDetail[]`→field-errors; X-Request-ID threaded. Tabela de testes cobre 503 vs 422, sufixos `0177`/`0178`, `0084` idempotência, precedência envelope-status vs transport-status, body vazio/não-json.

**Deviation:** Task 1.4.2 (normalizador de paginação) fechou como **no-op** — o trinaldo `List/Pages/All` + split page/cursor + encadeamento por-endpoint (`Page++` vs `NextCursor`) já existiam e passavam; o adaptador de offset interno é YAGNI (nenhuma entidade `packages`/`billing` existe até Phase 3). Contrarian de paginação confirmou: sem defeito.

### Epic 1.P1 + 1.R: Exemplar Organizations + remediação do gate — Done

**Exemplar** (`655a636`): `organizations_facade.go` — `List/Pages/All` page-based sobre `genledger.ClientWithResponses`, tipos gerados fora da superfície pública. É o template que a Phase 2 copia (estendido a CRUD em 2.1.0).

**Remediação** (`6bd5024`, `3d71601`, `9877c11`, `391a6a6`, `f587e6d` — wave `wqnkudtdr` PASS: 3 reviewers + 2 contrarian lenses): zerou os 9 findings do wave de fechamento (`wxcd3fcvo`) ANTES de propagar o exemplar aos ~10 recursos (corrigir o template é O(1); corrigir N cópias é O(n)). Fixes que viraram padrão nas fases seguintes: X-Request-ID no path de erro (money-path correlação); `IncludeDeleted`/filtros sem slot via `setQueryParam` req-editor (invariante #6); `LEDGER-0084`→`CodeIdempotency` via suffix map (invariante #2); teste do fallback `errUnrewindableBody` (invariante #4).

---

## Phase 2 — Ledger core (money path)

**Milestone:** Onboarding CRUD completo e o ciclo de vida de transação end-to-end funcionam contra o ledger, com counts e skips gated por settings.

### Epic 2.1: Recursos de onboarding — ✅ Done

**Goal:** organizations (write-exemplar), ledgers (+settings tri-bloco), accounts (+alias, sub-lists balances/operations cursor), assets, portfolios, segments, account-types, metadata-indexes funcionam via fachada (CRUD read+write), e2e-testados.
**Scope:** `entities/*_facade.go` (novos), extensões pontuais em `models/`, fachada sobre `internal/genledger`.
**Dependencies:** Phase 1
**Target:** midaz-sdk-golang
**Status:** Done (2026-07-01 — wave `w00eosgql`, 6 commits `efb7702`..`ecb1b4b`; review 4 + contrarian 4; 1 High + 2 Medium remediados em `00d6f4b`, wave `wo6cx0mvh`).

**Done when (Epic):** os 8 recursos têm fachada CRUD read+write e2e-testada contra httptest; sub-lists de account encadeiam por cursor; settings expõe tri-bloco; writes replay-safe; nenhum filtro dropado silenciosamente; gerados intocados; build/vet/test verdes. ✔️

**O que landou (fachadas em `entities/`):** `organizations` (CRUD, write-exemplar + helpers package-level), `ledgers` (+ `GetSettings`/`UpdateSettings` tri-bloco), `assets`, `portfolios`, `segments`, `account_types`, `accounts` (+ `GetByAlias` path + sub-lists cursor `ListBalances`/`ListOperations` + `BalancesAtTimestamp` slice), `metadata_indexes` (global, não-paginado).

**Deviations (informam Phases 3–4):**
- **[High, corrigido `wo6cx0mvh`] Cursor stop-condition:** `ListBalancesPages`/`ListOperationsPages` paravam em `!HasMore()`; o ramo heurístico page-based de `HasMore()` (`models/common.go:262`) dispara `true` numa página cursor terminal cheia que carrega campo `page`, com `NextCursor==""` → loop infinito re-fetchando página 1 (leitura money-path-adjacent). **Fix:** loops cursor param em `NextCursor == ""` direto. **Lição p/ Phase 3–4:** loop cursor NUNCA usa `HasMore()` como parada; só loop page-based (que avança `Page++` e é auto-corretivo).
- **[Medium×2, corrigido] Decode de valor monetário não-asserido:** testes de balances/`BalancesAtTimestamp` afirmavam só o `ID`, nunca o `available` (`decimal.Decimal`). Todo teste de leitura money-path DEVE asserir o valor decimal, não só a presença.
- **Filtros sem slot (invariante #6) — mapa real por recurso:** Assets `code/type/status` (todos via editor); Segments `name/status/include_deleted` (via editor); Portfolios `name/include_deleted` (via editor), `entity_id/status` (slot nativo); Ledgers `include_deleted` (via editor), `name/status` (slot); AccountTypes `key_value` (slot!), `name/include_deleted` (via editor); Accounts `holder_id/include_deleted` (via editor), 12 outros (slot). O vision de 2.1.a errou ("filtros mapeiam a params gerados") — verificar sempre contra o struct gerado, não contra a convenção.
- **Accounts — `models.BalancesListOpts` NÃO foi mutado:** é owned pelo path legado (`entities/balances.go`, `pkg/integrity/checker.go`). O sub-list de balances toma `models.CursorListOpts` direto (é o shape exato do param gerado); operations ganhou `models.AccountOperationsListOpts` novo (type/direction/route_id/route_code — reusar `OperationsListOpts` dropava filtros).
- **Ledger settings tri-bloco:** `models.LedgerSettings` estendido aditivamente (Accounting.RequireHolder + blocos Overrides + Tracer). O path legado agora é silenciosamente capaz de round-tripar os blocos novos (campos required default-zero no decode; input só ganhou pointers opcionais) — deixado intocado (coexistência).
- **metadata-indexes:** `entityName` é QUERY param no List (`entity_name`), PATH segment no Create/Delete. Vive no plano **Ledger** (não transaction como o legado sugeria).
- **Generated-parse footgun:** `Parse*Resp` de ops com resposta typed (ex. `GetAccountBalancesAtTimestampResp`) faz UNMARSHAL EAGER que valida `openapi_types.UUID` mesmo em HTTP 200 — fixtures de teste dessas ops PRECISAM de UUIDs reais; só as sub-lists `*Pagination`-wrapped (Items untyped) toleram ids sintéticos curtos.
- **YAGNI Count:** nenhum `Count{X}Resp` tipa `X-Total-Count`; `Pagination.HasMore()` cobre "tem mais". Count cortado da wave (revisitar em 2.3 se um endpoint exigir contagem explícita).

### Epic 2.2: Ciclo de vida de transação (money-write crown jewel)

**Goal:** Os quatro paths de create (`/json`, `/inflow`, `/outflow`, `/annotation`), `commit`/`cancel`/`revert`, get/list(cursor)/count e updates (transação + operation) funcionam via fachada, com idempotência `X-Idempotency`/`X-TTL` wired e replay-safe no path gerado, e `feesSkipped`/`tracerSkipped` refletidos na resposta.
**Scope:** `entities/transactions_facade.go` (novo) + `entities/idempotency.go` (novo, helper compartilhado), `models/transaction.go` (extensão aditiva), `pkg/sdkctx/` (knob de TTL), sobre `internal/genledger`.
**Dependencies:** Epic 2.1 (write-facade pattern, cursor-stop lição, `authRefreshRoundTripper`).
**Done when:** os 4 creates + commit/cancel/revert + get/list(cursor)/count + updates passam e2e; idempotência wired (paridade legada) e replay-safe; `feesSkipped`/`tracerSkipped` no model; nenhum filtro dropado; cursor termina em `NextCursor==""`; gerados intocados; build/vet/test verdes.
**Target:** midaz-sdk-golang
**Status:** ✅ Done (2026-07-01 — commits `d80e47e` idempotência, `f685784` creates, `2a589c4` lifecycle, `bfe9992` reads, `3cf5839` updates; success-gate defect corrigido `970db6f` (wave `wogt1dh0p`, review 2 + contrarian 2); response-amount assert `b0c2348`).

> **DECISÕES DE WAVE (supervisor, 2026-07-01, vs recon):**
> - **Idempotência é o LINCHPIN (money-path, terceiro rail).** O path gerado NÃO herda a auto-geração/injeção ctx→header do `*HTTPClient` legado (`injectContextHeaders` http.go:553, `ensureIdempotencyHeader` http_retry_response.go:1021 são do legado); o `authRefreshRoundTripper` só PRESERVA no replay via `req.Clone()`+`GetBody`, não CRIA a chave. Sem wiring, um create de transação sai SEM chave → retry de rede = 2ª mutação de balance (violação double-entry). Os 4 creates gerados têm `params.XIdempotency`+`params.XTTL` (nomes `X-Idempotency`/`X-TTL` confirmados — NUNCA `-Key`); commit/cancel/revert NÃO têm params. **Task 2.2.0 constrói o helper ANTES dos creates.**
> - **Success codes divergem:** os 4 creates = **200** (não 201); commit/cancel/revert = **201**. `isSuccess` (2xx) cobre ambos — não hardcodar.
> - **Reads decodam bytes crus em `models.*`** — o `Parse*Resp` de transação unmarshala em `genledger.Transaction`/`Operation` com `openapi_types.UUID` não-opcional → UUID-eager-validate (footgun da 2.1.c). List = **cursor** (para em `NextCursor==""`, lição 2.1, NUNCA `HasMore()`); filtros sem slot via `setQueryParam`. **Fixtures PRECISAM de UUIDs reais.**
> - **`/dsl`→`/json` DIFERIDO p/ Epic 5.2.** O cliente gerado NÃO tem op `/dsl`; o legado `/dsl` é wire path multipart separado; `/dsl`→`/json` é COMPORTAMENTO NOVO (adapter DSL→input). É açúcar ergonômico, não primitivo money-path. Override lane aberto.
> - **Skip-gating: escopo = REFLETIR, não REQUISITAR.** `models.Transaction` (transaction.go:57) dropa `FeesSkipped`/`TracerSkipped` que o gerado carrega (:720/731) → estender. REQUISITAR skip (intenção no body opaco) = YAGNI até um consumidor precisar. 422 `0490` chega como `*errors.Error` genérico (tipar = follow-up).
> - **Coexistência mantida:** `transactions_facade.go` construída + e2e-testada, NÃO wired em `client.X`. Gerados intocados. **SERIAL** (money-write, arquivos compartilhados). Padrão-base: `entities/accounts_facade.go` + helpers package-level.

#### Task 2.2.0: Idempotência no path de write gerado (linchpin money-path)
- [x] Done
**Context:** o path gerado não herda a idempotência do `*HTTPClient` legado (recon: `injectContextHeaders` http.go:553, `ensureIdempotencyHeader` http_retry_response.go:1021 são legado; o `authRefreshRoundTripper` só preserva no replay). `pkg/sdkctx` já tem `WithIdempotencyKey`/`IdempotencyKeyFromContext`/`WithoutAutoIdempotency`/`AutoIdempotencySuppressed` (sdkctx.go:59/73/103/113). Constante `idempotencyHeader="X-Idempotency"` (http.go:65). Os 4 creates gerados têm `params.XIdempotency`+`params.XTTL`; commit/cancel/revert não têm params. Server: default TTL 300s.
**Implementation vision:** helper package-level `resolveIdempotency(ctx, explicitKey string, autoGen bool) (key, ttl string)` na camada de fachada. Resolução (paridade legada): (1) chave explícita — `explicitKey` (input) OU `sdkctx.IdempotencyKeyFromContext(ctx)` — vence; (2) senão, se `autoGen && !sdkctx.AutoIdempotencySuppressed(ctx)`, gera `uuid.NewString()`; (3) senão vazio. TTL de knob novo `sdkctx.WithIdempotencyTTL(ctx, seconds)`+`IdempotencyTTLFromContext` (vazio = omitir X-TTL, server usa 300). Aplicação: ops com params (4 creates) → setar `params.XIdempotency=&key`/`params.XTTL=&ttl` quando não-vazios; ops sem params (commit/cancel/revert) → reqEditor `setHeader(k,v)` (irmão do `setQueryParam`). `autoGen=true` p/ creates (paridade: unsafe idempotente por default), `false` p/ actions (paridade `transactionActionContext`). Helper NÃO toca o body; chave estável sobrevive ao replay.
**Files:** Create `entities/idempotency.go` + `entities/idempotency_test.go`; Modify `pkg/sdkctx/sdkctx.go` (+`WithIdempotencyTTL`/`IdempotencyTTLFromContext`).
**Verification:** `go test ./entities/ ./pkg/sdkctx/ -run 'Idempotency' -count=1` — explícita>ctx>auto; `WithoutAutoIdempotency` suprime; TTL do ctx; header = `X-Idempotency`/`X-TTL`.
**Done when:** helper resolve chave/TTL com paridade legada, aplica via params (creates) e reqEditor (actions), headers `X-Idempotency`/`X-TTL`.

#### Task 2.2.1: Transactions facade — 4 create paths
- [x] Done
**Context:** `CreateTransaction{JSON,Inflow,Outflow,Annotation}WithBodyWithResponse(ctx, orgId, ledgerId, params, "application/json", body, reqEditors...)`→200→`genledger.Transaction`. Models SEPARADOS: `CreateTransactionInput` (transaction.go:262, `IdempotencyKey json:"-":312`), `CreateInflowInput`/`CreateOutflowInput`/`CreateAnnotationInput` (transaction_convenience.go:11/197/385) — inflow/outflow via `ToMap()`, json/annotation via `ToLibTransaction()` (mappers diferentes). `models.Transaction` dropa `FeesSkipped`/`TracerSkipped` do gerado (:720/731).
**Implementation vision:** `CreateJSON/CreateInflow/CreateOutflow/CreateAnnotation(ctx, orgID, ledgerID, input) (*models.Transaction, error)`. Cada: valida, `resolveIdempotency(ctx, input.IdempotencyKey, true)`→params, marshal do wire shape do input (o que cada endpoint espera — confirmar no TDD contra `api/ledger.openapi.yaml`)→`bytes.NewReader`→Create* gerado. Decode bytes crus→`models.Transaction`. **Estender `models.Transaction`** aditivamente com os campos money-path que o server manda e o SDK dropa (`FeesSkipped`, `TracerSkipped` + fee-legs/valor-líquido presentes em `genledger.Transaction` — ler o struct e espelhar). Sucesso=200.
**Files:** Create `entities/transactions_facade.go` + `_test.go`; Modify `models/transaction.go`.
**Verification:** `go test ./entities/ ./models/ -run 'TestTransactionsFacade_Create|Transaction' -count=1` — 4 paths mandam wire shape certo + `X-Idempotency` presente/estável no replay + flags de skip decodam.
**Done when:** 4 creates passam e2e, idempotentes e replay-safe, resposta expõe skip flags.

#### Task 2.2.2: Lifecycle — commit/cancel/revert
- [x] Done
**Context:** `Commit/Cancel/RevertTransactionWithResponse(ctx, orgId, ledgerId, transactionId, reqEditors...)`→201→Transaction, SEM params/body. Legado: revert retorna FILHO (`ParentTransactionID`, não muta original, transactions.go:962); **cancel sintetiza** `&Transaction{ID, Status:{Code:"CANCELED"}}` se body vazio/null (:1067-1091). Actions não-idempotentes por default (`transactionActionContext`:1099).
**Implementation vision:** `Commit/Cancel/Revert(ctx, orgID, ledgerID, transactionID) (*models.Transaction, error)`. Idempotência via 2.2.0 `autoGen=false` (só se caller passar chave no ctx → reqEditor `setHeader`). Cancel: 201 body vazio/`null` → sintetizar `models.Transaction{ID, Status: CANCELED}`. Sucesso=201.
**Files:** Modify `entities/transactions_facade.go` + `_test.go`.
**Verification:** `go test ./entities/ -run 'TestTransactionsFacade_(Commit|Cancel|Revert)' -count=1`.
**Done when:** as 3 ações passam e2e com semântica legada (revert-filho, cancel-sintetizado, non-idempotent por default).

#### Task 2.2.3: Read path — Get + List (cursor) + Count
- [x] Done
**Context:** `GetTransactionWithResponse`→200→Transaction; `GetAllTransactionsWithResponse(...params)`→200→`Pagination` (Items interface{}→unmarshal manual em `models.ListResponse[Transaction]`). List = CURSOR (`GetAllTransactionsParams`:1362 = Metadata/Limit/dates/SortOrder/Cursor, sem Page). `CountTransactionsByFiltersWithResponse` = HEAD (Route/Status/dates). Filtros de `models.TransactionsFilters` sem slot → `setQueryParam`.
**Implementation vision:** `Get` (decode bytes→models.Transaction, evita UUID-eager-validate); `List/Pages/All` **cursor** (Pages para em `NextCursor==""` — NUNCA `HasMore()`); filtros sem slot via `setQueryParam` (asset_code/status/source/destination/route/reference — nomes wire de `TransactionsFilters.ToQueryParams`); `Count` HEAD lendo `X-Total-Count`.
**Files:** Modify `entities/transactions_facade.go` + `_test.go`.
**Verification:** `go test ./entities/ -run 'TestTransactionsFacade_(Get|List|Count)' -count=1` — cursor encadeia ≥2 páginas e termina; filtros na query; count parseia X-Total-Count.
**Done when:** get/list(cursor)/count passam e2e; nenhum filtro dropado; cursor termina em `NextCursor==""`.

#### Task 2.2.4: Update transaction + update operation
- [x] Done
**Context:** `UpdateTransactionWithBodyWithResponse(...contentType, body)`→200→Transaction; `UpdateOperationWithBodyWithResponse(...operationId, contentType, body)`→200→**Operation**. Legado: `UpdateTransactionInput` = só Metadata+Description (transaction.go:1221), PATCH objeto inteiro `application/json` (NÃO merge-patch), payload não-vazio exigido. `UpdateTransactionOperation` (operations.go:343) PATCH tx-scoped.
**Implementation vision:** `UpdateTransaction(ctx, org, ledger, id, *UpdateTransactionInput) (*models.Transaction, error)` e `UpdateOperation(ctx, org, ledger, txID, opID, input) (*models.Operation, error)` via write-facade (`application/json`, NÃO merge-patch — paridade). Recusar payload vazio. UpdateOperation decoda `models.Operation`.
**Files:** Modify `entities/transactions_facade.go` + `_test.go` (+ `models/operation.go` só se faltar campo money-path do gerado).
**Verification:** `go test ./entities/ -run 'TestTransactionsFacade_Update' -count=1`.
**Done when:** update de transação e de operation passam e2e; payload vazio recusado.

**Done when (Epic 2.2):** 4 creates + commit/cancel/revert + get/list(cursor)/count + updates passam e2e; idempotência wired e replay-safe; `feesSkipped`/`tracerSkipped` no model; nenhum filtro dropado; cursor termina em `NextCursor==""`; gerados intocados; build/vet/test verdes. `/dsl` diferido p/ Epic 5.2. ✔️

> **Defeito money-path pego no gate (supervisor, 2026-07-01, wave `wogt1dh0p`):** os 8 writes roteavam pelo parser gerado `Parse{Op}Resp` (gate por status EXATO — creates 200 / actions 201 / updates 200); qualquer 2xx fora disso (202 async, drift OAS↔server) caía no ramo default que faz `json.Unmarshal` no `Error` gerado (`status *int64`), mas o body real de transação tem `status` OBJETO → unmarshal falha → write CONFIRMADO vira erro interno espúrio. `isSuccess` era dead code. **Fix:** rerotear os 8 writes pelos métodos lower-level (`...WithBody`/`Commit`/`Cancel`/`Revert`/`Update...WithBody`, todos retornam `*http.Response` cru) → `readRawResponse` (drena+fecha body) → `isSuccess(2xx)` como único gate → decode em `models.*`. Paridade com o legado (`StatusCode < 400`). RED capturado (create@201/@202, commit@200). Replay/idempotência/cancel-synthesis preservados; reads intocados.

> **Follow-up p/ cutover (Epic 5.1) — retrofit uniforme de idempotência:** as fachadas de write da 2.1 (onboarding Create/Update) E da 2.3 (operation-routes/transaction-routes create+update, asset-rates PUT) NÃO setam idempotência no path gerado (2.1 passa `params` vazio; routes/asset-rates não têm params de idempotência → precisariam de `setHeader` reqEditor). Retrofitar TODAS pro helper 2.2.0 (`resolveIdempotency`+`setHeader`) num passo uniforme antes/durante o cutover — o contrato do SDK é auto-idempotência em unsafe methods. Money-write (transações 2.2) já está wired. (asset-rate é PUT-upsert = idempotente por natureza da tupla from/to, menor risco; routes são POST sem dedup natural.)

### Epic 2.R: Hardening do baseline golangci (lint verde p/ o gate de make ci)

**Goal:** `golangci-lint run ./...` fica verde (0 issues) — hoje **57 issues** (recon 2026-07-01), 100% código à mão introduzido no branch de consolidação (não regressão do baseline v4.1.0 released; `generated: lax` já isenta `*.gen.go`). Estabelece baseline limpo ANTES de 2.3 e vira gate de wave dali em diante.
**Scope:** `entities/*_facade_test.go` (helpers de teste), `entities/transactions_facade.go`/`organizations_facade.go` (nolint bodyclose), style em `entities/*_facade*.go`, `internal/genledger/smoke_test.go`, `internal/cmd/specdowngrade/`.
**Dependencies:** Epic 2.2 (não mexer em money-path em voo).
**Target:** midaz-sdk-golang
**Status:** ✅ Done (2026-07-01 — wave `wr0d1vw13`, 6 commits `3f4d5ad`..`57c8de4`; review logic/test/security + contrarian no-behavior-change/no-over-suppression, ambos `defectFound:false` provados; 1 Low fechado `7d44962` (pin de assinatura do `NewClient` restaurado com nolint). `golangci-lint run ./...` = 0 issues verificado por mim; untouchables intocados; zero mudança de comportamento money-path).

> **Descoberta de processo (supervisor, 2026-07-01):** as waves 2.1/2.2 gatearam em `go build/vet/test` mas NÃO em `golangci-lint`. `go test` não vê função package-level não-usada (U1000) nem bodyclose. `.golangci.yml` tem `new:false` → o `make ci` terminal (Phase 5) tem ZERO tolerância. Debt acumulou invisível — mesma classe de falso-verde do defeito success-gate, um nível acima. **Correção: da wave 2.3 em diante o GREEN da harness inclui `golangci-lint run` nos pacotes tocados; a wave só retorna lint-clean.**

**Escopo exato (57 findings):**
- **entities/ (51):**
  - `unused: 8` — os construtores `newXFacade` (8 recursos) não são chamados: nem `client.X` (cutover diferido p/ 5.1), nem os testes (montam struct literal `&xFacade{ledger:...}`). **Fix (não `nolint`):** rotear os helpers `newTestXFacade` pelos construtores reais (`return newXFacade(newTestLedgerClient(t, srv))`) — limpa unused, exercita o seam, e 5.1 chama os mesmos construtores em `client.X`. **Estabelece o padrão p/ 2.3+.**
  - `bodyclose: 5` — 3 em `transactions_facade.go:165/180/196` (commit/revert/cancel via `readRawResponse`) são FALSO-POSITIVO: `readRawResponse` fecha via `defer resp.Body.Close()` (:63) — bodyclose não rastreia close através de helper. `//nolint:bodyclose // fechado em readRawResponse` justificado. Idem `organizations_facade.go:263` (helper de reads) e `auth_roundtripper_test.go:249`. **Provar o close em cada um antes do nolint — senão é leak real.**
  - style (~37): `errorlint` (`errors.As/Is`), `mnd` (nomear números mágicos — status codes de teste), `usestdlibvars` (`http.StatusX`), `revive` (`_` p/ param não-usado). Mecânico, zero mudança de comportamento.
- **internal/genledger/smoke_test.go (2):** `revive` (param `t`→`_`) + `staticcheck QF1011` (omitir tipo inferível). Trivial. NÃO é gerado (o gerado real é skip via `generated: lax`).
- **internal/cmd/specdowngrade (4):** `goimports` (fmt), `mnd` (3 mágico), e **2 gosec** — `G703` path-traversal (`main.go:231`) + `G306` perms WriteFile (`main.go:241`). **gosec = julgamento de segurança, não carimbo:** G306→apertar perms se o spec de saída é artefato de build; G703→validar path OU `#nosec G703` justificado se o path é input fixo repo-relativo de build. security-reviewer adjudica.

**Done when:** `golangci-lint run ./...` = 0 issues; nenhum gerado tocado à mão; nenhum comportamento money-path alterado (só test-helper routing + nolint justificado + style + gosec adjudicado); build/vet/test verdes; construtores `newXFacade` exercitados pelos testes.

### Epic 2.3: Routes, asset-rates, counts

**Goal:** Operation-routes, transaction-routes e asset-rates funcionam via fachada nova sobre `internal/genledger` (coexistência, não-wired); counts (HEAD→`X-Total-Count`) completos e endurecidos.
**Scope:** `entities/operation_routes_facade.go` + `transaction_routes_facade.go` + `asset_rates_facade.go` (novos); extensão dos facades 2.1 (Count); `entities/transactions_facade.go` (harden Count). Models já existem.
**Dependencies:** Epic 2.1 (write-facade pattern, cursor-stop, helpers), Epic 2.2 (`readRawResponse`, `parseTotalCountHeader`).
**Target:** midaz-sdk-golang
**Status:** ✅ Done (2026-07-01 — wave `w1z2n8xhn`, 4 commits `8ebc407` operation-routes, `7e99caf` transaction-routes, `4ad40ab` asset-rates, `a313fa6` readCount+counts; review logic/test/nil + contrarian wire-parity/cursor, ambos limpos; 9 agentes, ~884k tokens. Gate do supervisor PASS: build/vet/`golangci-lint`=0/test verdes; gerados+`entity.go`/`plane_clients.go`/`common.go` intocados (coexistência); 4 alegações de money-path re-derivadas do código pousado — (1) asset-rate rate/scale sem divisão por float, `json.Marshal` direto, `*decimal.Decimal` na leitura → sem truncamento; (2) cursor para em `NextCursor==""` nas 3 fachadas, `HasMore` só em comentários; (3) count error-path via `readCount`→`DecodeProblemJSON` mapeia HEAD-403 corpo-vazio p/ `*errors.Error`, não InternalError; (4) writes replay-safe via corpo rebobinável de `writeJSON`).

> **DECISÕES DE WAVE (supervisor, 2026-07-01, vs recon verificada):**
> - **Os arquivos `entities/{operation_routes,transaction_routes,asset_rates}.go` são LEGADO** (`e.httpClient.sendRequest`, struct `*Entity`, wired em `entity.go:270/277/281`) — shipam no v4.1.0. São a REFERÊNCIA COMPORTAMENTAL (mesmo server): a fachada nova reproduz o wire request equivalente. NÃO existe `*_facade.go` p/ eles ainda. Recon disse "tudo pronto" — FALSO; confundiu legado wired com fachada feita.
> - **Models já existem, ZERO criação:** `models.OperationRoute`/`CreateOperationRouteInput`/`UpdateOperationRouteInput`, `models.TransactionRoute`/`Create`/`Update` (`OperationRoutes []uuid.UUID` no body), `models.AssetRate`/`CreateAssetRateInput` (`Rate int`+`Scale int` fixed-point; PUT é upsert, sem UpdateInput). List opts `OperationRoutesListOpts`/`TransactionRoutesListOpts` existem. **Watch money-path:** o marshaling de asset-rate (`rate`/`scale`) TEM que bater byte-a-byte com o legado `asset_rates.go` (OAS declara `rate` como `number`) — asserir que a taxa não trunca.
> - **Idempotência DIFERIDA p/ o retrofit de cutover (Epic 5.1).** Os métodos gerados de create/PUT de routes/asset-rates NÃO têm params `XIdempotency`/`XTTL` (≠ os 4 creates de transação). Wire via `setHeader` seria possível, mas 2.1 (onboarding, análogo config-write) também deixou idempotência p/ o retrofit — fazer igual mantém UM retrofit uniforme (onboarding+routes+asset-rates) no cutover, em vez de espalhar setHeader por-fachada. Money-path idempotente (transações) já está wired na 2.2. **Writes SEGUEM replay-safe via `bytes.NewReader` (GetBody)** — replay pós-401 independe de idempotência.
> - **Success codes:** Create routes = POST→**200**; Update = PATCH→**200**; Delete = DELETE→**204** (Out sem body); asset-rate = PUT→**200** (upsert). Get/List = GET→200. `isSuccess`(2xx) cobre; writes decodam via `readRawResponse`+`decodeOne` como a 2.2 (nunca o parser typed com status-exato).
> - **Counts:** o header `X-Total-Count` NÃO é campo do `Count{X}Resp` gerado (segue verdade da 2.1) — lê-se do `resp.HTTPResponse.Header` via `parseTotalCountHeader` (http.go:834). O parser gerado `ParseCount*Resp` só faz `json.Unmarshal` se `Content-Type` contém "json" → defeito ESTREITO de error-path: content-type json + body vazio → parser erra → `...WithResponse` devolve `(nil,err)` → fachada devolve InternalError em vez do status real. **Harden via `readCount` sobre o método lower-level raw** (mesma filosofia do fix 2.2). Endpoints de count: org/ledger/account/asset/portfolio/segment + transactions (`CountTransactionsByFilters` tem params status/transactionRoute); **NÃO há count p/ routes/asset-rates**.
> - **Coexistência mantida:** fachadas novas construídas + e2e-testadas, NÃO wired em `client.X`. Gerados intocados. Padrão-base: `accounts_facade.go` (CRUD+cursor), `transactions_facade.go` (`readRawResponse`/`decodeOne`), `organizations_facade.go` (helpers). **Gate da wave inclui `golangci-lint run` (lição 2.R): test-helpers roteiam pelos construtores reais; wave só retorna com `golangci-lint run ./...` = 0.**

#### Task 2.3.1: Operation-routes facade
- [ ] Done
**Context:** gerados (`internal/genledger/ledger.gen.go`): `ListOperationRoutes`(:3227,+WithResponse:16024), `CreateOperationRouteWithBody`(:3239), `GetOperationRouteByID`(:3275), `UpdateOperationRouteWithBody`(:3287), `DeleteOperationRoute`(:3263). `ListOperationRoutesParams`(:1250) = limit/start_date/end_date/sort_order/cursor (cursor pagination). Verbos: List/Get GET→200, Create POST→200, Update PATCH→200, Delete DELETE→204. Models `models.OperationRoute`/`CreateOperationRouteInput`/`UpdateOperationRouteInput` existem; `OperationRoutesListOpts` existe. Referência comportamental: legado `entities/operation_routes.go` (`operationRoutesEntity`).
**Implementation vision:** novo `entities/operation_routes_facade.go`, struct `operationRoutesFacade{ledger *genledger.ClientWithResponses}` + `newOperationRoutesFacade`. `List/Pages/All` cursor (para em `NextCursor==""`, NUNCA `HasMore()`); `Get`; `Create`/`Update` via write-facade (`json.Marshal(input)`→`bytes.NewReader`→`...WithBodyWithResponse` ou lower-level+`readRawResponse`+`decodeOne`, `isSuccess` 2xx); `Delete` (204, sem body). Idempotência NÃO wired (diferida 5.1). Filtros sem slot via `setQueryParam` se `OperationRoutesListOpts` tiver algum. Test helper `newTestOperationRoutesFacade` roteia pelo construtor real (lição 2.R).
**Files:** Create `entities/operation_routes_facade.go` + `_test.go`.
**Verification:** `go test ./entities/ -run 'TestOperationRoutesFacade' -count=1`; `golangci-lint run ./entities/` = 0. Cursor encadeia ≥2 páginas e termina; create manda o body certo; update PATCH; delete 204.
**Done when:** CRUD+list-cursor de operation-routes passa e2e; wire equivalente ao legado; replay-safe; lint-clean.

#### Task 2.3.2: Transaction-routes facade
- [ ] Done
**Context:** gerados: `ListTransactionRoutes`(:3539,+WithResponse:16251), `CreateTransactionRouteWithBody`(:3551), `GetTransactionRouteByID`(:3587), `UpdateTransactionRouteWithBody`(:3599), `DeleteTransactionRoute`(:3575). `ListTransactionRoutesParams`(:1337) = mesmo set cursor. Verbos idem 2.3.1. Body de create embute `OperationRoutes []uuid.UUID` (compõe operation-routes por ID; `models.CreateTransactionRouteInput.OperationRoutes`). Models existem; `TransactionRoutesListOpts` existe. Referência: legado `entities/transaction_routes.go`.
**Implementation vision:** novo `entities/transaction_routes_facade.go`, mesma forma da 2.3.1. Watch: o create serializa `operationRoutes` como array de UUID (não objetos) — bater com o legado; `models.CreateTransactionRouteInput` já stasheia parse-err de UUID no construtor.
**Files:** Create `entities/transaction_routes_facade.go` + `_test.go`.
**Verification:** `go test ./entities/ -run 'TestTransactionRoutesFacade' -count=1`; `golangci-lint run ./entities/` = 0. Create manda `operationRoutes` como UUIDs; CRUD+cursor passa.
**Done when:** CRUD+list-cursor de transaction-routes passa e2e; body de create manda os UUIDs de operation-route; wire equivalente ao legado; lint-clean.

#### Task 2.3.3: Asset-rates facade
- [ ] Done
**Context:** gerados: `CreateOrUpdateAssetRateWithBody`(:2987) **PUT**→200 (upsert por tupla from/to), `GetAllAssetRatesByAssetCode`(:3011) GET→200 (params: `to[]` array, limit, start_date, end_date, sort_order, cursor), `GetAssetRateByExternalID`(:3023) GET→200. OAS schema (`api/ledger.openapi.yaml` ~378-461): request `from`(str), `to`(str), `rate`(**number/double**), `scale`(int opt), `source`(str opt), `ttl`(int opt), `externalId`(uuid opt), `metadata`. `models.AssetRate` (Rate `*decimal.Decimal`, Scale `*int`) e `CreateAssetRateInput` (Rate `int`, Scale `int`) existem; SEM UpdateInput (PUT é upsert). Referência: legado `entities/asset_rates.go`.
**Implementation vision:** novo `entities/asset_rates_facade.go`. `CreateOrUpdateAssetRate` (PUT upsert, write-facade), `GetAssetRate` (by externalID), `ListByAssetCode`/`Pages`/`All` (cursor, filtro `to[]` — verificar se é slot nativo em `GetAllAssetRatesByAssetCodeParams` ou via `setQueryParam`). **CRÍTICO money-path:** o marshaling de `rate`/`scale` TEM que reproduzir o legado `asset_rates.go` byte-a-byte (a OAS quer `rate` como number; o input é int+scale fixed-point → confirmar como o legado converte e mandar idêntico). Asserir no teste que a taxa decodada bate com a enviada (sem truncar).
**Files:** Create `entities/asset_rates_facade.go` + `_test.go`.
**Verification:** `go test ./entities/ -run 'TestAssetRatesFacade' -count=1`; `golangci-lint run ./entities/` = 0. PUT upsert manda from/to/rate/scale; list by-asset filtra `to[]`; rate round-trip sem truncar.
**Done when:** upsert + get-by-externalID + list-by-asset(cursor) passam e2e; wire de rate/scale idêntico ao legado; lint-clean.

#### Task 2.3.4: Counts — readCount helper + harden transactions + onboarding
- [ ] Done
**Context:** counts são HEAD→204 com total em `X-Total-Count` (NÃO é campo do `Count{X}Resp`). `parseTotalCountHeader`(entities/http.go:834) já existe. `transactions_facade.Count`(:365) já lê o header MAS via `CountTransactionsByFiltersWithResponse` — o parser `ParseCountTransactionsByFiltersResp` só unmarshala se Content-Type contém "json"; um erro com content-type json + body vazio faz o parser errar→`(nil,err)`→InternalError (defeito estreito error-path). Métodos gerados (raw `*http.Response`): `CountOrganizations`(:2171), `CountLedgers`(:2627), `CountAccounts`(:2855), `CountAssets`(:3071), `CountPortfolios`(:3347), `CountSegments`(:3443), `CountTransactionsByFilters`(:3707, params status/transactionRoute).
**Implementation vision:** helper package-level `readCount(resp *http.Response, err error) (int, error)` (irmão de `readRawResponse`): err→InternalError; `!isSuccess`→`DecodeProblemJSON` (trata body vazio); senão `parseTotalCountHeader`. Rerotear `transactions_facade.Count` p/ o método lower-level raw + `readCount` (endurece o error-path). Adicionar `Count(ctx, ids, [opts]) (int, error)` aos facades 2.1 org/ledger/account/asset/portfolio/segment via `readCount`. **Decisão YAGNI-de-2.1 revisitada:** incluir os 6 counts de onboarding — os endpoints existem e consumidores de paginação precisam do total; override lane p/ o Fred se achar over-scope.
**Files:** Modify `entities/transactions_facade.go` (Count) + os 6 `*_facade.go` de onboarding + helper em `entities/transactions_facade.go` ou `organizations_facade.go`; Test: os respectivos `_test.go`.
**Verification:** `go test ./entities/ -run 'Count' -count=1`; `golangci-lint run ./entities/` = 0. Success 204 lê X-Total-Count; erro com body vazio+content-type json → RFC 9457, NUNCA InternalError.
**Done when:** transactions Count endurecido; 6 counts de onboarding funcionam; error-path de body-vazio mapeado a `*errors.Error`; lint-clean.

**Done when (Epic 2.3):** operation-routes/transaction-routes/asset-rates via fachada nova (CRUD+cursor+PUT-upsert) passam e2e em coexistência; counts completos e endurecidos (raw-response, X-Total-Count); models reusados sem regressão; wire equivalente ao legado (asset-rate rate/scale sem truncar); idempotência diferida p/ 5.1; gerados intocados; `golangci-lint run ./...` = 0; build/vet/test verdes.

---

## Phase 3 — Ledger new domains (CRM + fees + billing + crypto)

**Milestone:** Holders/instruments/composition, fees (packages/estimates), billing e encryption/protection funcionam via a fachada. Todos os recursos copiam o write-facade pattern + `setQueryParam` de filtros + a lição cursor-stop da Phase 2.

### Epic 3.1: Holders, Instruments, Composition

**Goal:** Holders (re-homed no ledger, path-based), instruments (+related-parties), e composition (holder+account+instrument numa chamada, envelope não-atômico `{account,instrument,instrumentError}`) funcionam.
**Scope:** `entities/`, `models/` (holders/instruments/composition); idempotência via `X-Idempotency`/`X-TTL` (invariante #5 — **NUNCA** `X-Idempotency-Key`).
**Dependencies:** Phase 2
**Done when:** CRUD de holders/instruments passa; composition expõe `instrumentError` sem engolir (sem rollback server-side); CRM enforcement (422 `0491` requireHolder) tipado.
**Target:** midaz-sdk-golang
**Status:** Pending

### Epic 3.2: Fees (packages/estimates) + Billing

**Goal:** Packages (definições de fee, offset-paginated), estimates (dry-run), billing-packages e billing-calculate funcionam.
**Scope:** `entities/`, `models/` de fees e billing.
**Dependencies:** Phase 2
**Done when:** CRUD de packages passa (offset pagination via trinaldo — aqui o adaptador de offset adiado em 1.4.2 pode finalmente ser necessário); `/estimates` retorna cálculo dry-run; billing-calculate retorna results+summary; códigos numéricos de fee tipados.
**Target:** midaz-sdk-golang
**Status:** Pending

### Epic 3.3: Encryption + Protection

**Goal:** Encryption (`/provision`, `/status`) e protection (`/audit`) funcionam.
**Scope:** `entities/`, `models/` de encryption/protection.
**Dependencies:** Phase 2, Epic 1.4 (decoder — nota: `pkg.HTTPError` foi unificado no `Error` RFC 9457 por Plano A, então não há envelope especial a tratar aqui).
**Done when:** Provision/status/audit passam; 404 = legacy mode tratado.
**Target:** midaz-sdk-golang
**Status:** Pending

---

## Phase 4 — Tracer plane

**Milestone:** Os cinco primitivos do tracer funcionam via a fachada, com auth Bearer compartilhado e X-API-Key opcional.

### Epic 4.1: Client do tracer + rules + limits

**Goal:** O plano tracer autentica (Bearer compartilhado ou X-API-Key), e rules (CEL, lifecycle DRAFT→ACTIVE→INACTIVE) e limits (+usage) funcionam.
**Scope:** fachada sobre `internal/gentracer`, `entities/`, `models/` de rules/limits.
**Dependencies:** Phase 1 (Client do tracer já constrói)
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

## Phase 5 — Ergonomia, cutover, docs, contract tests

**Milestone:** Helpers ergonômicos completos; **cutover do accessor + deleção do legado** (fim da coexistência, invariante #7); docs/exemplos/mapping atualizados; testes de contrato regenerados; `make ci` verde.

### Epic 5.1: Cutover do accessor + deleção do legado

**Goal:** As fachadas `entities/*_facade.go` (Phases 2–4) são plugadas em `client.X`; os serviços legados (`entities/*.go` sem `_facade`) e seu `*HTTPClient`/retry são deletados num passo atômico. Fim da coexistência.
**Scope:** `entities/entity.go`, `entities/plane_clients.go`, remoção do path legado, `midaz.go`.
**Dependencies:** Phases 2–4 (todas as fachadas existem e passam e2e)
**Done when:** `client.X` roteia pras fachadas; nenhum serviço legado sobra; `examples/` e consumidores migram numa passada; build/test verdes.
**Target:** midaz-sdk-golang
**Status:** Pending

### Epic 5.2: Helpers ergonômicos

**Goal:** Builders fluentes (com `FieldErrors`), DSL de transação e `WaitForSettlement` (poll de balance, não de status) funcionam.
**Scope:** `models/transaction_dsl.go`, builders, novo helper de settle em `pkg/transaction` ou `entities/`.
**Dependencies:** Phases 2–4
**Done when:** `WaitForSettlement` polla balance com backoff/timeout e documenta que 201 ≠ liquidado; DSL aponta `/json`; builders cobrem os recursos novos.
**Target:** midaz-sdk-golang
**Status:** Pending

### Epic 5.3: Docs, exemplos, mapping, contract tests

**Goal:** `README.md`, `docs/README.md`, `docs/mapping/`, `docs/examples.md` e o `mass-demo-generator` refletem a superfície nova; os `*_contract_regression_test.go` são regenerados/validados; cobertura ≥80% na lógica crítica nova; `make ci` verde.
**Scope:** `docs/`, `examples/`, `README.md`, testes de contrato.
**Dependencies:** Phases 2–4 + Epic 5.1
**Done when:** exemplos rodam contra o stack novo; mapping atualizado; `make demo-data` funciona; `make ci` verde; testes de contrato batem com as specs versionadas.
**Target:** midaz-sdk-golang
**Status:** Pending

---

## Phase 6 — Streaming consumer *(opcional / decisão de produto)*

### Epic 6.1: Consumidor Kafka/CloudEvents

**Goal:** Helper de consumo de eventos reusando os payloads públicos de `pkg/streaming/events` do midaz.
**Scope:** novo pacote `pkg/streaming` no SDK.
**Dependencies:** decisão de produto (não é requisito da consolidação — emissão é producer-only server-interna hoje)
**Done when:** A definir se/quando priorizado.
**Target:** midaz-sdk-golang
**Status:** Pending

---

## Self-review

- **Spec coverage:** Ledger ~107 ops por Phases 2–3 (onboarding 2.1 ✅, txn/balances/ops 2.2, routes/rates/counts 2.3, holders/instruments/composition 3.1, fees/billing 3.2, encryption/protection 3.3). Tracer 31 ops por Phase 4 (rules/limits 4.1, reservations/validations 4.2, audit 4.3). Fundação Phase 1 ✅. Cutover/docs Phase 5. Streaming opcional Phase 6. Sem gap conhecido.
- **Vagueness scan:** a onda detalhada é a Phase 2; Epic 2.1 está fechado com deviations concretas; 2.2/2.3 seguem epic-level (serão detalhadas contra o código real quando forem a onda corrente — rolling wave). Sem "appropriate"/"TBD" na onda detalhada.
- **Contract consistency:** os 8 invariantes travados acima são a fonte única; toda fase referencia-os em vez de redefinir. `*errors.Error` / `DecodeProblemJSON` (erro), trinaldo `List/Pages/All` (paginação), write-facade + `bytes.NewReader` (write), `authRefreshRoundTripper` (replay), `setQueryParam` (filtros sem slot) — todos definidos na Phase 1 e reusados adiante.
- **Phase boundaries:** cada fase termina em software compilável e testável (Phase 1 lista orgs; Phase 2 fecha o money path; Phase 5 corta o legado e roda `make ci`).
- **Verification plausibility:** comandos apontam paths reais (`entities/`, `models/`, `pkg/errors`, `pkg/config`, `internal/gen*`).
