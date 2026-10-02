# Arquitetura

Aplicação pessoal de finanças: um backend em Go (API + canal de conversa + jobs), um dashboard em React e um Postgres. Os dados vêm do que a pessoa registra no chat (Telegram ou WhatsApp, com IA) e do Open Finance (Meu Pluggy).

```
Telegram / WhatsApp ──┐                                   ┌── Postgres
                      ├─► backend Go ──► usecase ──► ports ──► infra (db, pluggy, gemini, ...)
Dashboard (React) ────┘     (http, chat, app)                └── Pluggy / Gemini / Telegram / Evolution
```

## Camadas do backend (`backend/internal/`)

| Pacote | Papel | Pode depender de |
| --- | --- | --- |
| `domain` | Entidades e regras puras: `Purchase`, `Payment`, `Account`, `Goal`, `MonthTotals`, `Clock`, `OwnTransferMatcher`... Sem I/O. | nada do projeto |
| `domain/ports` | **Só interfaces** (contratos): leituras por contexto (`BudgetReader`, `ReviewReader`...), `LedgerStore`, `ExternalStore`, `Messenger`, `Coach`, `OpenFinanceProvider`... | `domain` |
| `usecase/*` | Casos de uso e regras de cálculo, um pacote por contexto (abaixo). | `domain`, `ports`, `format` |
| `infra/*` | Adaptadores: `db` (Postgres), `http` (API e webhook), `pluggy`, `gemini`, `telegram`, `evolution`, `logo`. | `domain`, `ports`, `usecase` |
| `chat` | Interpreta mensagens do dono e responde (comandos, registro, importação). | `usecase`, `ports` |
| `settings` | Configurações editáveis no dashboard (valor salvo > ambiente > padrão), segredos cifrados. | `domain` |
| `config` | Lê e valida o ambiente (`Load(overrides)`). | — |
| `app` | **Composição**: o único lugar que conhece todas as peças (`cmd/main.go` só chama `app.Run`). | tudo |
| `format` | Formatação compartilhada (`FormatBRL`). | — |

Regra de dependência: `domain` ← `ports` ← `usecase` ← `infra`/`chat` ← `app`. Nada em `domain` ou `usecase` importa `infra`.

### Contextos de `usecase/`

```
ledger/        registro de despesas, receitas e transferências (texto, foto, extrato), exportação CSV, relatório mensal
openfinance/   sincronização das transações, contas, instituições e investimentos
planning/      orçamento (fixas/parceladas/variáveis), projeção e metas
review/        revisão de gastos (sugestões de corte) e economia realizada
coach/         contexto sanitizado e validação das respostas do Coach (IA) e da classificação sugerida
balances/      saldos por banco, severidades e texto do /saldos
insights/      fachada de leitura (Insights): projeção, revisão, metas, saldos e resumo semanal
```

Dependências permitidas entre contextos (verificadas por `usecase/architecture_test.go`): `review → planning`; `coach → planning, review`; `insights → planning, review, balances`. `ledger`, `openfinance`, `planning` e `balances` não dependem de outro contexto. Código compartilhado vai para `domain/` ou um pacote utilitário, nunca de um contexto para outro.

### Banco (`infra/db`)

Um repositório por contexto sobre o mesmo pool: `LedgerRepo` (lançamentos, contas e investimentos do Open Finance, transferências próprias), `TotalsRepo`, `TransactionsRepo`, `AccountsRepo`, `PortfolioRepo`, `BudgetRepo`, `ReviewRepo`, `GoalsRepo`, `CoachRepo`, `ClassifyRepo`, `SettingsStore`. `DashboardRepo` compõe os de leitura e satisfaz `ports.DashboardReader`. As consultas ficam em `*_queries.go`, uma por contexto; o que é compartilhado (mês de um pagamento, normalização de descrição `expenseKey`) fica em `queries_common.go`.

### HTTP (`infra/http`)

Cada contexto tem o seu `*_api.go` com os handlers e um `register*` com as rotas; `api.go` só cria as dependências e chama os registros. Guardas reutilizáveis em `guards.go`: `jsonOnly` (corpo JSON + mesma origem, contra CSRF), `sameOriginOnly` e `decodeJSON` (limite de corpo). Erros de validação voltam como `apiError` e são respondidos em `respondErr`.

## Fluxos

**Sincronização do Open Finance** (`app/sync.go` → `usecase/openfinance`): a cada minuto o laço relê a configuração; se há credenciais e o intervalo venceu, busca por item (`pluggy.FetchItem`): conector (banco, cor, logo), contas e transações dos últimos `SYNC_LOOKBACK_DAYS` dias. Cada transação passa por `pluggy/mapper.go` (descarta o que não é gasto nem renda, classifica) e por `SyncOne`: já existe? concilia com lançamento manual? senão cria. É idempotente (o ID da origem é único).

**Mensagem do chat** (`chat.Handler`): comandos fixos (`/sync`, `/resumo`, `/saldos`, confirmações) são tratados sem IA; o resto vai para `ledger.AnalyzeExpense`, que usa o Gemini para entender texto, imagem ou extrato e grava os lançamentos.

**Requisição do dashboard**: nginx (`frontend/nginx.conf`) serve o React e repassa `/api` ao backend na mesma origem (sem CORS). A sessão é um cookie assinado; cada rota passa por limite de taxa e sessão (`routes.protected`).

**Configurações** (`settings`): valor salvo no dashboard > variável de ambiente > padrão. Valem na hora resumo, sincronização, nomes, Pluggy e plano pago do Coach; o resto pede reinício. Segredos são cifrados com `APP_SECRET_KEY`/`APP_SECRET_KEY_FILE` (veja [as decisões](decisoes/)).

## Frontend (`frontend/src/`)

React + TypeScript + React Query + Recharts, organizado por funcionalidade:

```
src/
  features/<contexto>/   balances, settings, budget, projection, review, goals, coach, categorize, investments,
                         transactions, overview, spending, compare, auth. Cada uma tem a página, api.ts (tipos e hooks
                         do contexto), components/, lib/ (lógica pura, com testes) e styles.css quando precisa.
  shared/                api/ (request, totais, contas), components/ (QueryState, StatTile, gráficos...), lib/ (formatação, meses, tema)
  shell/Layout.tsx       menu lateral e estrutura das telas logadas
  index.css              tokens (cores, tema) e base compartilhada
```

Regra (verificada por `architecture.test.ts`): uma funcionalidade só importa dela mesma e de `shared/`; a única exceção é `coach → review` (o Coach comenta as sugestões da Revisão). Para compartilhar algo entre funcionalidades, mova para `shared/`.
