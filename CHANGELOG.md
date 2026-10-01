# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/).

## [Não lançado]

### Adicionado
- **Saldo real dos investimentos** via `GET /investments` do Pluggy: cada sincronização grava as posições (tipo, subtipo, nome do produto, saldo líquido e valor bruto) e o saldo do dia, formando o histórico do patrimônio. Posições que somem do Pluggy (resgatadas) ficam inativas com saldo zero. Falha ou ausência de investimentos não conta como erro da sincronização.
- API: `GET /api/portfolio` (posições, total e total por tipo) e `GET /api/portfolio/history?from=&to=` (saldo ao fim de cada mês, `null` antes do primeiro registro).
- Migration `007_create_investments.sql`.
- Tela **Investimentos** no dashboard: patrimônio investido hoje (saldo real por tipo e por produto), evolução do saldo e o fluxo de aplicações e resgates (aplicado, resgatado e líquido acumulado por mês, em 6, 12 ou 24 meses).
- Telas **Gastos** (despesas do mês por categoria, forma de pagamento e conta/cartão), **Comparações** (categoria no mês escolhido contra o anterior e evolução em 6, 12 ou 24 meses), **Transações** (lista com filtros por mês, tipo, categoria, forma de pagamento, conta e busca, com paginação) e **Contas** (saldo e limite usado dos cartões) no dashboard.
- **Dashboard em React** (`frontend/`: Vite, TypeScript, TanStack Query, React Router e Recharts) com login, tema claro/escuro e a tela **Visão geral**: receitas, despesas, saldo do mês, "em conta" e investimentos com a variação sobre o mês anterior, e o gráfico de receitas e despesas dos últimos 12 meses (com visão em tabela). O mês vai na URL (`?mes=AAAA-MM`).
- Serviço `web` no Docker Compose: nginx sem privilégios que serve o front e repassa `/api` ao app (mesma origem, sem CORS), com CSP restritiva e demais cabeçalhos de segurança. Porta configurável por `WEB_PORT` (padrão 8080), publicada só em `127.0.0.1`.
- Alvos `make front-dev`, `make front-test` e `make front-build`.
- **API de leitura do dashboard** sob `/api` (resumo do mês, série temporal, gastos por categoria, forma de pagamento e conta/cartão, investimentos, transações com filtros e paginação, contas), com agregação feita no SQL.
- Autenticação do dashboard: `POST /api/login` com a senha de `DASHBOARD_PASSWORD` (mínimo de 12 caracteres) emite um cookie de sessão assinado (`HttpOnly`, `SameSite=Strict`, `Secure` atrás de HTTPS). Sem a variável, a API não é montada.
- Contas e cartões do Open Finance agora são persistidos (tabela `accounts`: tipo, nome, 4 últimos dígitos, saldo, limite e limite disponível), e cada pagamento sincronizado aponta para a conta de origem (`payments.account_id`). Base para "gasto por conta/cartão" e "saldo real" no dashboard.
- Migration `006_create_accounts.sql`.
- Integração com **Open Finance via Meu Pluggy** (opcional): importa contas e cartões automaticamente, ao subir e a cada `SYNC_INTERVAL_HOURS`, usando `GET /v2/transactions` (o endpoint v1 é descontinuado em 2026-12-31).
- Variáveis `PLUGGY_CLIENT_ID`, `PLUGGY_CLIENT_SECRET`, `PLUGGY_ITEM_IDS`, `SYNC_INTERVAL_HOURS` e `SYNC_LOOKBACK_DAYS`. Sem elas, nada muda.
- Comando `/sync` (Telegram) e `sincronizar` (qualquer canal) para sincronizar sob demanda.
- Sincronização idempotente por ID de origem e **conciliação** com lançamentos manuais equivalentes, evitando contar duas vezes.
- Pagamento de fatura e transferência entre contas próprias são ignorados; aplicação e resgate de investimento viram Transferência.
- Migration `005_add_payment_external_id.sql`.
- Canal **Telegram** como alternativa ao WhatsApp, selecionado por `CHANNEL=telegram`. Usa a Bot API oficial por long polling (sem URL pública) e suporta texto, foto de recibo, PDF de extrato, exportação de CSV e relatório mensal.
- Variáveis `CHANNEL`, `TELEGRAM_BOT_TOKEN` e `TELEGRAM_CHAT_ID`. `CHANNEL` é `whatsapp` por padrão.
- Perfil `whatsapp` no Docker Compose: Evolution API e Redis só sobem com `COMPOSE_PROFILES=whatsapp`.
- Pacote `internal/chat` com a lógica de conversa independente de canal.

### Alterado
- A porta `3000` do app no `docker-compose.yml` passou a escutar apenas em `127.0.0.1`. O webhook da Evolution API usa a rede interna do compose e não é afetado.
- O rate limit por IP passou a considerar `X-Real-IP` quando a requisição vem de um proxy da rede privada.
- `ports.OpenFinanceProvider.FetchTransactions` virou `FetchItem` e devolve também as contas; `ExistsExternalID` virou `LinkExternalAccount`, que além de checar vincula a conta a transações sincronizadas antes da migration 006.
- O código Go foi movido para `backend/` (o module path não mudou) para abrir espaço ao front-end. `Dockerfile`, `docker-compose.yml` e `Makefile` foram ajustados; os comandos `make` continuam os mesmos e a pasta das migrations agora é `backend/migrations/`.
- A configuração só exige `EVOLUTION_*` e `OWNER_PHONE` quando `CHANNEL=whatsapp`.
- O webhook do WhatsApp virou um adapter fino sobre `internal/chat`; o comportamento visível é o mesmo, exceto que falhas ao baixar e ao decodificar um documento agora geram uma única mensagem de erro.
- `ports.Messenger` perdeu `FetchImageBase64` (detalhe da Evolution API) e `SendDocument` passou a receber `[]byte` em vez de base64.
- `httpserver.NewServer` expõe só `/health`; as rotas do WhatsApp são adicionadas por `MountWhatsApp`.

### Corrigido
- Bancos novos não recebiam as migrations 002 a 004 (o compose montava só a 001), o que causava `column "kind" of relation "purchases" does not exist` ao registrar a primeira despesa. Agora toda a pasta `migrations/` é montada no `initdb`.

### Segurança
- Investimentos: o app descarta titular, CNPJ do emissor, código e número da posição (dados pessoais segundo o Pluggy) na leitura; só o nome do produto, truncado em 120 caracteres, é guardado.
- API do dashboard: senha comparada em tempo constante, sessão assinada com HMAC e chave aleatória por boot, login limitado a 5 tentativas por minuto por IP, login/logout exigem JSON e mesma origem (CSRF), parâmetros validados por lista fechada (nunca interpolados no SQL, e `%`/`_` da busca são escapados), respostas com `Cache-Control: no-store` e erros internos genéricos (o detalhe vai só para o log). A API nunca expõe `external_id` nem o texto bruto original.
- Contas: o app lê do Pluggy apenas nome, tipo, saldo, limites e os 4 últimos dígitos do número; CPF, nome do titular e número completo são descartados na leitura e nunca chegam ao banco nem aos logs.
- Open Finance: erros do cliente Pluggy nunca incluem credenciais nem URL, o cursor de paginação só é aceito se apontar para o próprio Pluggy (a requisição leva a `apiKey`) e `PLUGGY_ITEM_IDS` é validado como UUID.
- As portas do Postgres e do Redis no `docker-compose.yml` passaram a escutar apenas em `127.0.0.1`.
- No Telegram, só o `TELEGRAM_CHAT_ID` configurado, em conversa privada, é atendido, e o token é removido dos erros de rede para não vazar em logs.

### Migração
- Aplique também a migration 007 (investimentos): `docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/007_create_investments.sql`. O histórico do patrimônio começa na primeira sincronização depois dela (o Pluggy não informa saldos passados).
- Aplique a migration 006 (depende da 005): `docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/006_create_accounts.sql`. O próximo `/sync` (ou a sincronização automática) preenche a conta das transações já importadas dentro de `SYNC_LOOKBACK_DAYS`.
- Bancos já criados precisam aplicar a migration 005 antes de ligar o Open Finance: `docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/005_add_payment_external_id.sql`.
- Bancos já criados com o compose antigo não reexecutam o `initdb`: aplique à mão as migrations que faltarem, por exemplo `docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/002_add_kind_to_purchases.sql` (e as seguintes).
- Quem já usa WhatsApp deve adicionar `COMPOSE_PROFILES=whatsapp` ao `.env`. Sem isso, `docker compose up` não sobe a Evolution API.
