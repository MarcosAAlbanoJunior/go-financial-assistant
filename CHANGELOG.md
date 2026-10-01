# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/).

## [Não lançado]

### Adicionado
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
- O código Go foi movido para `backend/` (o module path não mudou) para abrir espaço ao front-end. `Dockerfile`, `docker-compose.yml` e `Makefile` foram ajustados; os comandos `make` continuam os mesmos e a pasta das migrations agora é `backend/migrations/`.
- A configuração só exige `EVOLUTION_*` e `OWNER_PHONE` quando `CHANNEL=whatsapp`.
- O webhook do WhatsApp virou um adapter fino sobre `internal/chat`; o comportamento visível é o mesmo, exceto que falhas ao baixar e ao decodificar um documento agora geram uma única mensagem de erro.
- `ports.Messenger` perdeu `FetchImageBase64` (detalhe da Evolution API) e `SendDocument` passou a receber `[]byte` em vez de base64.
- `httpserver.NewServer` expõe só `/health`; as rotas do WhatsApp são adicionadas por `MountWhatsApp`.

### Corrigido
- Bancos novos não recebiam as migrations 002 a 004 (o compose montava só a 001), o que causava `column "kind" of relation "purchases" does not exist` ao registrar a primeira despesa. Agora toda a pasta `migrations/` é montada no `initdb`.

### Segurança
- Open Finance: erros do cliente Pluggy nunca incluem credenciais nem URL, o cursor de paginação só é aceito se apontar para o próprio Pluggy (a requisição leva a `apiKey`) e `PLUGGY_ITEM_IDS` é validado como UUID.
- As portas do Postgres e do Redis no `docker-compose.yml` passaram a escutar apenas em `127.0.0.1`.
- No Telegram, só o `TELEGRAM_CHAT_ID` configurado, em conversa privada, é atendido, e o token é removido dos erros de rede para não vazar em logs.

### Migração
- Bancos já criados precisam aplicar a migration 005 antes de ligar o Open Finance: `docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/005_add_payment_external_id.sql`.
- Bancos já criados com o compose antigo não reexecutam o `initdb`: aplique à mão as migrations que faltarem, por exemplo `docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/002_add_kind_to_purchases.sql` (e as seguintes).
- Quem já usa WhatsApp deve adicionar `COMPOSE_PROFILES=whatsapp` ao `.env`. Sem isso, `docker compose up` não sobe a Evolution API.
