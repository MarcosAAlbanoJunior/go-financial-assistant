# Spec: Refatoração de estrutura e boas práticas

- **Status:** implementada (fases 1 a 8); a fase 9 (dinheiro como tipo) segue em aberto, aguardando decisão. Veja "Resultado e descobertas" ao fim.
- **Origem:** varredura do projeto feita em 02/10/2026 (código, testes, documentação, frontend e processo)
- **Natureza:** refatoração **sem mudança de comportamento**. Nenhuma funcionalidade nova, nenhuma rota, tela ou mensagem muda.
- **Esta spec é aberta de propósito:** quem implementar deve fazer a sua própria varredura (seção 9) e registrar o que achar, além do que está listado aqui.

## 1. Objetivo

Deixar o projeto fácil de entender, alterar e testar à medida que cresce, aplicando o básico de boas práticas:

- cada contexto de negócio no seu lugar (pacotes por assunto, não por tipo de arquivo);
- arquivos e funções pequenos, com um motivo só para mudar;
- abstrações onde hoje há repetição, e nenhuma onde não há necessidade;
- explicações de algoritmo e decisões em documentação, não espalhadas em comentários;
- um processo que impeça a regressão (formatação, lint e CI).

**Não objetivos:** reescrever regra de negócio, trocar de framework ou biblioteca, mudar o esquema do banco (exceto o que a fase 9 decidir), mudar a API HTTP, o texto das mensagens ou o visual das telas.

## 2. Regras de execução (valem para todas as fases)

1. **Comportamento idêntico.** Cada fase termina com `go test ./...`, `go vet ./...`, `staticcheck ./...`, `npx tsc -b`, `npx oxlint` e `npx vitest run` verdes. Se um teste precisar mudar, deve ser só por causa de mover/renomear (nunca para "fazer passar").
2. **Commits pequenos e revisáveis**, um por passo lógico (ex.: "refactor: Dividir api.go por contexto"). Nada de commit gigante misturando mover arquivos com mudar lógica. **Mover e alterar não andam juntos:** primeiro move (diff mecânico), depois, em outro commit, altera.
3. **Testes de integração de banco** rodam contra um banco descartável, nunca o de uso: crie `finassist_scratch`, aplique todas as migrations de `backend/migrations/` e rode `TEST_DATABASE_URL=<url do scratch> go test ./...`; apague o banco ao fim.
4. **Sem abstração especulativa.** Só extraia interface, tipo ou pacote quando houver pelo menos dois usos reais ou um ganho claro de teste. Registre a justificativa no commit.
5. **Compatibilidade de dados.** Nada de migration nesta spec, salvo a fase 9 (e só depois de decidida).
6. **Idioma:** manter o padrão atual (identificadores em inglês; comentários, mensagens de erro e logs em português).
7. **Verificação manual ao fim de cada fase grande** (seção 8): subir o app e passar pelo checklist de fumaça.

## 3. Estado atual (medido em 02/10/2026)

| Tema | Medida |
| --- | --- |
| Arquivos Go de produção | ~110, 11,3 mil linhas; testes ~9,6 mil linhas |
| `internal/usecase` | 27 arquivos num só pacote, de 6 contextos sem relação |
| `internal/infra/db/dashboard_queries.go` | 841 linhas: SQL de orçamento, revisão, metas, Coach, carteira, classificação |
| `PostgresPurchaseRepository` | 51 métodos implementando ~13 interfaces |
| `internal/infra/http/api.go` | 794 linhas, 21 handlers de assuntos diferentes |
| `internal/domain/ports/dashboard.go` | 312 linhas: entidades (`Goal`, `Decision`, `Account`...) misturadas a contratos; `DashboardReader` embute 6 interfaces |
| `cmd/main.go` | 193 linhas numa função `main` |
| Funções acima de 60 linhas | `main` 193, `BuildBalances` 97, `createGoal` 86, `ExportCSV.Execute` 84, `config.Load` 72, `coachAnalyze` 72, `toExternal` 69, `BuildBudget` 67, `processInstallment` 66, `webhook Handle` 66 |
| Repetição em HTTP | checagem "JSON + mesma origem" 11x; `json.NewDecoder(http.MaxBytesReader(...))` 11x |
| Strings mágicas | `"BANK"`/`"CREDIT"` em 6 pontos; `'EXPENSE'`/`'INCOME'`/`'TRANSFER'` em ~20 pontos de SQL/Go |
| `time.Now()` direto | 35 usos fora de testes |
| Estado global | `config.overrides` (variável de pacote); `time.Local` sobrescrito no `main` |
| Avisos do `staticcheck` | tipo `paymentModel` sem uso; `Sprintf` desnecessário em `chat/reply.go`; erro capitalizado em `config.go`; campos mortos em `settings_api_test.go` |
| `gofmt` | 3 arquivos fora do padrão: `usecase/analyze_expense.go`, `process_query.go`, `process_transfer.go` |
| CI / linter | sem `.github/`; `make lint` usa `golangci-lint`, que não está instalado nem configurado (`.golangci.yml` ausente) |
| Frontend | `api/types.ts` 403 linhas e `api/client.ts` 201 (todos os contextos juntos); `components/` com 30+ arquivos planos; `index.css` 523 linhas; `Transactions.tsx` 272 e `Projection.tsx` 232 linhas |
| Documentação | README de 458 linhas (instalação + operação + design); não existe `docs/` |

## 4. Fases

Cada fase tem critérios de aceite. Podem ser entregues como PRs separados, nesta ordem (as primeiras são de baixo risco e destravam as seguintes).

### Fase 1: Higiene e processo

- Rodar `gofmt -w` nos 3 arquivos listados e corrigir os 4 avisos do `staticcheck`.
- Criar `backend/.golangci.yml` (mínimo: `govet`, `staticcheck`, `errcheck`, `gofmt`, `unused`, `ineffassign`, `misspell` com locale BR desligado) e fazer o `make lint` funcionar (documentar a instalação no README ou usar `go run` de versão fixa).
- Criar `.github/workflows/ci.yml`: backend (`gofmt -l` sem saída, `vet`, `staticcheck`, `go test ./...` com serviço Postgres e `TEST_DATABASE_URL`) e frontend (`npm ci`, `tsc -b`, `oxlint`, `vitest run`, `build`).
- Adicionar `CONTRIBUTING.md` curto: como rodar tudo, regra de commits, regra de idioma, regra "mover ≠ alterar".

**Aceite:** CI verde em PR; `make lint` roda localmente; nenhuma saída de `gofmt -l .`.

### Fase 2: HTTP, repetição e divisão do `api.go`

- Extrair helpers em `infra/http`:
  - `decodeJSON(w, r, &v, maxBytes) bool` (limite de corpo + erro 400 padronizado);
  - middleware `requireJSON` (Content-Type `application/json` + mesma origem) usado nas escritas; o handler deixa de repetir a checagem.
- Dividir [api.go](../../backend/internal/infra/http/api.go) por contexto, um arquivo e um `register(mux, protected)` por contexto, no padrão que `goals_api.go` e `coach_api.go` já seguem:
  `auth_api.go` (login/logout/me), `summary_api.go` (summary, timeseries, breakdown), `investments_api.go` (investments, portfolio), `budget_api.go` (budget, projection, regras), `transactions_api.go`, `accounts_api.go`. O `mountAPI` deve só criar as dependências e chamar cada `register`.
- Quebrar handlers acima de ~50 linhas (`createGoal`, `coachAnalyze`, `testConnection`) em funções menores (validar → executar → responder).

**Aceite:** nenhum arquivo de handler acima de ~250 linhas; os testes de API passam sem alteração de comportamento; as rotas registradas são exatamente as mesmas (comparar a lista de `mux.Handle` antes e depois).

### Fase 3: Dados, tipos e repositórios por contexto

- Criar tipos para o que hoje é texto solto: `domain.AccountType` (`Bank`, `Credit`) e usá-lo em `ports.Account`, `ExternalAccount` e nas comparações; centralizar os literais de `kind` do SQL em constantes (ou parâmetros) em vez de strings repetidas.
- Dividir [dashboard_queries.go](../../backend/internal/infra/db/dashboard_queries.go) em arquivos por contexto (`budget_queries.go`, `review_queries.go`, `goals_queries.go`, `coach_queries.go`, `portfolio_queries.go`, `classify_queries.go`, `transactions_queries.go`), **primeiro só movendo** as funções.
- Em seguida, separar o repositório: em vez de um `PostgresPurchaseRepository` com 51 métodos, um tipo por contexto (`BudgetRepo`, `GoalsRepo`, `PortfolioRepo`, `LedgerRepo`...) que compartilham o pool. Manter construtores pequenos (`db.NewGoalsRepo(pg)`).
- Isolar a normalização de descrição (`expenseKey`, hoje regex em SQL, em `dashboard_queries.go`) em um único lugar nomeado e testado (constante ou função SQL), com comentário curto apontando para o documento de regras (fase 7).

**Aceite:** nenhum arquivo em `infra/db` acima de ~350 linhas; nenhum tipo com mais de ~15 métodos públicos; testes de integração do banco passam contra o banco descartável.

### Fase 4: Domínio e ports

- Mover entidades e objetos de valor de `domain/ports/dashboard.go` (e arquivos vizinhos) para `domain` (`Goal`, `Decision`, `Account`, `Position`, `MonthTotals`, `Transaction`...). `ports` fica só com **interfaces** (contratos).
- Dividir `DashboardReader` em readers por contexto (`TransactionReader`, `PortfolioReader`, `AccountReader`, `GoalStore`, ...), cada consumidor depende só do que usa. `Insights` e os handlers recebem as interfaces mínimas.
- Atualizar os fakes dos testes (`fakeReader`, `mockPurchaseRepo`) para implementar só o necessário; fakes menores indicam que a divisão acertou.

**Aceite:** `ports/` sem structs de dados; `fakeReader` do pacote http não implementa métodos que o teste não usa.

### Fase 5: Composição (`main`) e configuração

- Criar `internal/app` com funções pequenas e nomeadas: `openDatabase`, `loadSettings`, `buildSync`, `buildChannel` (Telegram/WhatsApp), `buildJobs`, `mountDashboard`. `cmd/main.go` fica com ~30 linhas (ler config, montar, executar, encerrar).
- Remover o estado global de `config`: `config.Load(overrides)` recebe o mapa por parâmetro, em vez de `SetOverrides` + variável de pacote. Tratar `time.Local` num único ponto documentado (ou passar o `*time.Location` onde é usado).
- Dividir `config.Load` (72 linhas) em `loadApp`, `loadChannel`, `loadOpenFinance` etc. Padronizar mensagens de erro (minúsculas).
- Introduzir um `Clock` (`interface { Now() time.Time }`) onde o tempo for regra de negócio (saldos, vencimentos, resumo, sincronização) e injetá-lo; os testes passam a usar um relógio fixo. **Não** é preciso trocar todos os 35 usos: priorizar os que têm regra de negócio.

**Aceite:** `main()` com no máximo ~40 linhas; nenhuma variável de pacote mutável em `config`; testes de config/relógio passam; um teste de composição mínimo sobe o `app` com dependências falsas.

### Fase 6: `usecase` por contexto

Depois das fases 3 e 4 (para não mexer duas vezes nas mesmas importações), separar `internal/usecase` em subpacotes por contexto. Proposta (ajustar se a leitura do código sugerir algo melhor, registrando o motivo):

```
internal/usecase/
  ledger/        registro de despesas, receitas e transferências (process_*, analyze_expense, export_csv, monthly_report)
  planning/      orçamento, projeção, metas
  insights/      revisão, economia realizada, resumo semanal (digest)
  balances/      saldos por banco, formatação do /saldos
  openfinance/   sincronização
  coach/         Coach e seu histórico
```

- Código compartilhado (`median`, parsing de parcela, `FormatBRL`) vai para um pacote utilitário pequeno e sem dependências de contexto (ex.: `internal/usecase/shared` ou, melhor, `internal/money` e `internal/textutil`), nunca de um contexto para outro.
- Dividir as funções longas listadas na seção 3 (`BuildBalances`, `ExportCSV.Execute`, `BuildBudget`, `processInstallment`, `toExternal`...) em passos nomeados.

**Aceite:** nenhum pacote de contexto importa outro pacote de contexto (apenas `domain`, `ports` e utilitários); nenhuma função de produção acima de ~60 linhas (exceções justificadas em comentário); todos os testes passam.

### Fase 7: Documentação

- Criar `docs/arquitetura.md`: camadas, contextos, fluxo de uma sincronização, fluxo de uma mensagem do chat, diagrama simples (ASCII ou Mermaid).
- Criar `docs/regras-de-calculo.md`: como funcionam projeção (renda, fixas, parcelas), revisão (detectores e limites), saldos (severidades, "desatualizado", saldo devedor), metas, economia realizada. **Mover para lá** os blocos longos de comentário de algoritmo (ex.: `usecase/projection.go` ~linha 70, `db/dashboard_queries.go` ~linha 302, `db/open_finance_repository.go` ~linha 128), deixando no código só uma linha com o essencial e o link.
- Criar `docs/decisoes/` com ADRs curtos (contexto, decisão, consequências), no mínimo: fatura exibida como "saldo devedor"; parcela datada no mês da fatura; segredos cifrados com chave mestra fora do banco; precedência banco > ambiente > padrão nas configurações; Pix entre contas próprias ignorados.
- Enxugar o README: instalação, uso e operação ficam; design e explicações longas viram links para `docs/`. Manter o CHANGELOG como está.

**Aceite:** nenhum comentário de algoritmo com mais de ~5 linhas no código sem justificativa; README abaixo de ~300 linhas; links entre os documentos válidos.

### Fase 8: Frontend por funcionalidade

- Reorganizar para `src/features/<contexto>/` com `api.ts` (hooks do contexto), `types.ts`, `components/`, `Page.tsx` e `lib/` do contexto (ex.: `features/balances`, `features/settings`, `features/budget`, `features/review`, `features/goals`, `features/coach`, `features/investments`, `features/transactions`). `src/shared/` para o que é comum (formatação, `QueryState`, layout, tema, `request`).
- Dividir `api/types.ts` e `api/client.ts` por contexto; o `request`/`postJSON` fica em `shared/api`.
- Dividir `index.css` por funcionalidade (um CSS importado pelo componente/página) e manter só tokens e base global em um arquivo.
- Usar `QueryState` em todas as telas que hoje repetem o bloco de carregando/erro à mão (ex.: `Overview`).
- Extrair a lógica de filtros de `Transactions.tsx` e `Projection.tsx` para hooks (`useTransactionFilters`, `useProjectionScenarios`).

**Aceite:** nenhum componente ou página acima de ~200 linhas sem motivo; sem import "para cima" entre `features` (uma não importa de outra, só de `shared`); build, `tsc`, `oxlint` e testes verdes; telas idênticas (comparar com o checklist da seção 8).

### Fase 9 (opcional, exige decisão): dinheiro como tipo

Hoje os valores são `DECIMAL(14,2)` no banco e `float64` no Go; somas e comparações acumulam erro de arredondamento (há comparações com `0,005` espalhadas).

- Introduzir `money.Money` (centavos, `int64`) com formatação e soma seguras, usado em `domain`, `usecase`, API (continuar serializando como número decimal para não quebrar o front) e SQL (`NUMERIC` já é exato).
- **Pré-requisito:** decidir com o dono do projeto: migrar tudo de uma vez ou por contexto, e se a API passa a enviar centavos (mudança de contrato do front).
- Não iniciar sem essa decisão registrada em um ADR.

**Aceite (se feita):** nenhum `float64` representando dinheiro em `domain`/`usecase`; testes de borda de arredondamento; resultados das telas idênticos.

## 5. Estrutura-alvo (resumo)

```
backend/
  cmd/main.go                     # ~30 linhas
  internal/
    app/                          # composição (fase 5)
    config/                       # leitura e validação, sem estado global
    domain/                       # entidades e regras puras (fase 4)
    ports/                        # só interfaces
    usecase/{ledger,planning,insights,balances,openfinance,coach}/
    infra/{db,http,pluggy,telegram,evolution,gemini,logo}/   # db e http divididos por contexto
    settings/                     # já isolado; manter
    money/, textutil/             # utilitários sem contexto
docs/{arquitetura.md, regras-de-calculo.md, decisoes/, specs/}
frontend/src/{features/<contexto>/, shared/}
.github/workflows/ci.yml
```

## 6. Ordem e dependências

`1 → 2 → 3 → 4 → 5 → 6 → 7 → 8`, com a 9 à parte. As fases 2 e 8 são independentes entre si (backend × frontend) e podem andar em paralelo. A 6 depende de 3 e 4. A 7 pode começar assim que a 6 estabilizar os nomes, mas os ADRs podem ser escritos antes.

## 7. Riscos e como evitá-los

| Risco | Mitigação |
| --- | --- |
| Quebrar comportamento sem perceber | Regra 1 e 2: mover antes de alterar; testes verdes a cada commit; comparar a lista de rotas e o `go list ./...` antes/depois |
| Refatoração gigante impossível de revisar | PRs por fase, commits pequenos |
| Pacotes de contexto acoplados de novo | Verificar importações na fase 6 (`go list -deps` ou um teste de arquitetura simples que falha se um contexto importar outro) |
| Abstração demais | Regra 4: só com dois usos reais |
| Banco de uso afetado por testes | Regra 3: sempre banco descartável |

## 8. Verificação

**Automática (a cada fase):**

```bash
cd backend
gofmt -l .                      # sem saída
go vet ./...
staticcheck ./...
go test ./...
# integração (banco descartável):
docker compose exec -T postgres psql -U finassist -d postgres -c "CREATE DATABASE finassist_scratch"
for f in migrations/*.sql; do docker compose exec -T postgres psql -q -U finassist -d finassist_scratch < $f; done
TEST_DATABASE_URL=postgres://...finassist_scratch go test ./...
docker compose exec -T postgres psql -U finassist -d postgres -c "DROP DATABASE finassist_scratch"
cd ../frontend && npx tsc -b && npx oxlint && npx vitest run && npm run build
```

**Fumaça manual (ao fim das fases 2, 5, 6 e 8):** subir o app (`docker compose up -d --build`), entrar no dashboard e passar por todas as telas (Painel, Visão geral, Transações, Gastos, Classificar, Orçamento, Revisão, Metas, Coach, Projeção, Comparações, Investimentos, Configurações); clicar em Sincronizar; abrir e salvar uma configuração comum; no Telegram, `/saldos`, `/resumo` e `/sync`. Nada deve mudar de aparência ou de número.

## 9. Requisitos em aberto: descobertas de quem implementar

A varredura acima é um ponto de partida, **não o limite**. Antes de cada fase, quem for implementar deve fazer uma passada própria na área tocada e acrescentar aqui o que encontrar.

**O que procurar (lista, não exaustiva):** funções ou arquivos grandes; código duplicado (três repetições já pedem abstração); `if`/`switch` repetidos sobre o mesmo tipo; nomes enganosos; comentários que repetem o código ou que explicam "por que" demais (candidatos a documentação); erros ignorados (`//nolint:errcheck`); `panic` ou `os.Exit` fora do `main`; dependências cíclicas ou "para cima"; variáveis de pacote mutáveis; `context` não propagado; goroutines sem encerramento; SQL com concatenação onde parâmetros bastam; testes frágeis (dependem de relógio, ordem ou rede); código morto; configurações lidas em vários lugares; acessibilidade e estados vazios/de erro no frontend; imports não usados e dependências sem uso em `package.json`/`go.mod`.

**Como registrar:** acrescente uma linha na tabela abaixo no mesmo PR que tratar (ou decidir não tratar) o item.

| # | Onde | Achado | Gravidade (alta/média/baixa) | Decisão (corrigido na fase X / registrado para depois / descartado e por quê) |
| --- | --- | --- | --- | --- |
| | | | | |

**Regra de escopo:**
- Corrija na própria fase se for pequeno, estiver na área que você já está mexendo, não mudar comportamento e tiver teste cobrindo.
- Se for grande, mudar comportamento ou tocar outra área, **registre na tabela** como "para depois", com uma frase sobre a solução sugerida; não faça junto.
- Se discordar de algo desta spec (um pacote proposto, a ordem, um limite de tamanho), **registre a discordância e o motivo** em vez de seguir cegamente ou ignorar em silêncio.

**Perguntas em aberto para o dono do projeto** (responder antes ou durante a fase indicada):
1. Fase 6: os nomes e a fronteira dos subpacotes de `usecase` fazem sentido? (ex.: `ledger` vs `registro`; `insights` agrupar revisão, economia e resumo)
2. Fase 5: o `Clock` injetado deve valer só para regras de negócio ou o projeto quer padronizar tudo?
3. Fase 8: manter `features/` com tudo dentro (tipos, hooks, componentes, página) ou prefere separar `pages/` de `components/` globais?
4. Fase 9: dinheiro em centavos agora, depois ou nunca? Se agora, a API muda de contrato?
5. Fase 1: o CI deve rodar também o build da imagem Docker?

## 10. Definição de pronto

- Todas as fases aceitas (a 9 só se decidida) e seus critérios cumpridos.
- CI verde; `gofmt`, `vet`, `staticcheck`, `tsc`, `oxlint` sem avisos.
- Nenhuma mudança de comportamento percebida no checklist de fumaça.
- Tabela da seção 9 preenchida e sem itens de gravidade alta sem decisão.
- `docs/arquitetura.md`, `docs/regras-de-calculo.md` e os ADRs existem e batem com o código.
- CHANGELOG com uma entrada "Interno" resumindo a refatoração.


## 11. Resultado e descobertas (preenchido na implementação)

**Fases 1 a 8 concluídas** em `refactor/estrutura`, com os testes verdes a cada commit (`go test` com banco descartável, `golangci-lint`, `tsc`, `oxlint`, `vitest`) e o app rodando (rotas da API idênticas: 40 antes e depois do `api.go`).

Desvios em relação ao texto da spec:

- **Fase 3:** os repositórios por contexto são tipos separados (`LedgerRepo`, `BudgetRepo`...) compostos em `DashboardRepo`, em vez de um tipo por interface em pacotes separados; `ports.PurchaseRepository` foi dividido em `LedgerStore` + `ExternalStore`.
- **Fase 5:** `Clock` só nos casos de uso com regra de tempo (`AnalyzeExpense`, `SyncOpenFinance`, `DigestJob`); ficam com `time.Now()` três funções puras de `ledger` (`processExportCSV`, `resolveQueryMonth`, `previousMonth`).
- **Fase 6:** `balances` ficou separado de `insights`; a fachada `Insights` (`insights/`) e `FormatBRL` em `internal/format`. Teste de dependências em `usecase/architecture_test.go`.
- **Fase 7:** README em 318 linhas (meta: ~300); detalhes das telas, API, Coach e uso do chat viraram `docs/*.md`.
- **Fase 8:** `shell/Layout.tsx` fora de `features/` e `shared/` (é a casca de navegação); CSS só das funcionalidades claramente isoladas foi para `styles.css` (orçamento, simulador, transações e chips ficaram em `index.css` por serem usados por mais de uma tela).

| # | Onde | Achado | Gravidade | Decisão |
| --- | --- | --- | --- | --- |
| 1 | `cmd/main.go` | `os.Exit(0)` dentro de `connectWhatsApp` cortava o encerramento limpo | média | corrigido na fase 5 (devolve erro) |
| 2 | `config.go` | `OWN_NAMES` só era lido se o Open Finance estivesse configurado | baixa | registrado: o laço de sincronização já lê do serviço de configurações; vale mover a leitura para fora do bloco do Pluggy |
| 3 | `usecase/ledger` | três funções puras ainda usam `time.Now()` direto | baixa | registrado para depois |
| 4 | `infra/db` | nenhuma consulta de transações/agrupamento tem teste de integração dedicado; os testes ficam todos em `dashboard_queries_test.go` (735 linhas) | média | registrado: dividir o arquivo de testes por contexto, como as consultas |
| 5 | `frontend` | `Transactions.tsx` (256 linhas) e `Projection.tsx` (216) ainda passam de ~200 | baixa | registrado: extrair `FilterBar` e a lista de cenários |
| 6 | `frontend` | `index.css` ainda tem 339 linhas | baixa | registrado: mover orçamento/simulador/transações/chips para CSS das suas funcionalidades |
| 7 | `settings_api.go` | `putSettings` mistura confirmar senha, validar, salvar, auditar e procurar Pix próprios | média | registrado: extrair um serviço `SettingsService` fora do handler |
| 8 | geral | Dinheiro em `float64` (fase 9) | alta | **aberto: precisa de decisão do dono** (centavos agora, depois ou nunca; contrato da API) |
