# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/).

## [Não lançado]

### Adicionado
- Canal **Telegram** como alternativa ao WhatsApp, selecionado por `CHANNEL=telegram`. Usa a Bot API oficial por long polling (sem URL pública) e suporta texto, foto de recibo, PDF de extrato, exportação de CSV e relatório mensal.
- Variáveis `CHANNEL`, `TELEGRAM_BOT_TOKEN` e `TELEGRAM_CHAT_ID`. `CHANNEL` é `whatsapp` por padrão.
- Perfil `whatsapp` no Docker Compose: Evolution API e Redis só sobem com `COMPOSE_PROFILES=whatsapp`.
- Pacote `internal/chat` com a lógica de conversa independente de canal.

### Alterado
- A configuração só exige `EVOLUTION_*` e `OWNER_PHONE` quando `CHANNEL=whatsapp`.
- O webhook do WhatsApp virou um adapter fino sobre `internal/chat`; o comportamento visível é o mesmo, exceto que falhas ao baixar e ao decodificar um documento agora geram uma única mensagem de erro.
- `ports.Messenger` perdeu `FetchImageBase64` (detalhe da Evolution API) e `SendDocument` passou a receber `[]byte` em vez de base64.
- `httpserver.NewServer` expõe só `/health`; as rotas do WhatsApp são adicionadas por `MountWhatsApp`.

### Corrigido
- Bancos novos não recebiam as migrations 002 a 004 (o compose montava só a 001), o que causava `column "kind" of relation "purchases" does not exist` ao registrar a primeira despesa. Agora toda a pasta `migrations/` é montada no `initdb`.

### Segurança
- As portas do Postgres e do Redis no `docker-compose.yml` passaram a escutar apenas em `127.0.0.1`.
- No Telegram, só o `TELEGRAM_CHAT_ID` configurado, em conversa privada, é atendido, e o token é removido dos erros de rede para não vazar em logs.

### Migração
- Bancos já criados com o compose antigo não reexecutam o `initdb`: aplique à mão as migrations que faltarem, por exemplo `docker compose exec -T postgres psql -U finassist -d finassist < migrations/002_add_kind_to_purchases.sql` (e as seguintes).
- Quem já usa WhatsApp deve adicionar `COMPOSE_PROFILES=whatsapp` ao `.env`. Sem isso, `docker compose up` não sobe a Evolution API.
