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
| 2 | Money path completo: onboarding CRUD + ciclo de transação (json/inflow/outflow/annotation + commit/cancel/revert) + balances/operations/routes/asset-rates + counts | 2.1, 2.2, 2.3 | **Detailed** (2.1 Done; 2.2 é a próxima onda) |
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
**Status:** Done (2026-07-01 — wave `w00eosgql`, 6 commits `efb7702`..`ecb1b4b`; review 4 + contrarian 4; 1 High + 2 Medium remediados na wave `wo6cx0mvh`).

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

### Epic 2.2: Ciclo de vida de transação + balances + operations

**Goal:** Os quatro paths de create (`/json`, `/inflow`, `/outflow`, `/annotation`), `commit`/`cancel`/`revert`, balances (split `Balance` compact vs `BalanceHistory`) e operations funcionam; skips (`TransactionSkip{fees,tracer}`) honrados só quando `settings.overrides` habilita, com 422 `0490` claro caso contrário.
**Scope:** `entities/transactions*.go` + fachada nova, `models/transaction*.go`, balances/operations sobre `internal/genledger`.
**Dependencies:** Epic 2.1 (write-facade pattern, cursor-stop lição, settings tri-bloco).
**Done when:** criar/commitar/cancelar/reverter passa e2e; a resposta expõe fee legs + `amount` inflado/líquido; `feesSkipped`/`tracerSkipped` refletidos; `/dsl` mantido só como atalho para `/json`; writes replay-safe (X-Idempotency estável); nenhum filtro dropado.
**Target:** midaz-sdk-golang
**Status:** Pending *(próxima onda a detalhar — money-write crown jewel, escrutínio máximo, isolada. Elaborar contra o código real das fachadas 2.1 + ops de transação geradas antes de lançar.)*

### Epic 2.3: Routes, asset-rates, counts

**Goal:** Operation-routes, transaction-routes, asset-rates (create via `PUT .../asset-rates` com `from`/`to` no body) e counts via HEAD funcionam.
**Scope:** `entities/`, `models/` dos recursos acima.
**Dependencies:** Epic 2.1
**Done when:** CRUD passa; asset-rates create manda códigos no body; counts parseiam `X-Total-Count` (revisita o YAGNI Count de 2.1 se necessário).
**Target:** midaz-sdk-golang
**Status:** Pending

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
