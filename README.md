# Go Financial Assistant

Assistente financeiro pessoal via **WhatsApp ou Telegram** (você escolhe o canal). Envie mensagens de texto, fotos de recibos ou extratos bancários em PDF para registrar despesas, entradas e transferências automaticamente. O assistente utiliza IA (Google Gemini) para interpretar as transações e armazená-las em um banco de dados PostgreSQL.

## Como funciona

No WhatsApp você envia a mensagem para **si mesmo**; no Telegram você conversa com o **seu próprio bot**. Texto descrevendo um gasto, uma foto de recibo ou o PDF do extrato bancário — e o assistente registra as transações automaticamente.

**Exemplos de mensagens:**
- `"gastei 45 reais no almoço no pix"`
- `"netflix 55 reais todo mês todo dia 15"`
- `"cancelar netflix"`
- `"recebi 6000 reais de salário"`
- `"coloquei 2000 no cofrinho"`
- `"quanto gastei em março?"`
- `"exportar meus gastos de março"`
- Foto de um recibo ou nota fiscal
- PDF do extrato bancário (Itaú e outros)

O assistente classifica cada transação em três categorias:

| Tipo | Descrição | Exemplo |
|------|-----------|---------|
| **Despesa** | Gasto real de dinheiro | Almoço, Netflix, conta de luz |
| **Entrada** | Dinheiro recebido | Salário, freelance, reembolso |
| **Transferência** | Movimentação entre contas próprias | Aplicação no cofrinho, resgate de CDB |

Transferências são excluídas dos totais de despesas e entradas — evitando que aplicações no cofrinho distorçam o resumo financeiro.

## Tecnologias

- **Go** — aplicação principal
- **Google Gemini** — análise e interpretação das mensagens
- **Evolution API** — integração com WhatsApp (somente no canal WhatsApp)
- **Telegram Bot API** — integração com Telegram (somente no canal Telegram)
- **PostgreSQL** — armazenamento das despesas
- **Redis** — cache da Evolution API (somente no canal WhatsApp)
- **Docker / Docker Compose** — infraestrutura

## Pré-requisitos

- [Docker](https://www.docker.com/) com Docker Compose
- Chave de API do [Google Gemini](https://aistudio.google.com/app/apikey)
- Conta no WhatsApp **ou** no Telegram

## Configuração

### 1. Clone o repositório

```bash
git clone https://github.com/MarcosAAlbanoJunior/go-financial-assistant.git
cd go-financial-assistant
```

### 2. Escolha o canal e configure as variáveis de ambiente

Copie o arquivo de exemplo:

```bash
cp .env.example .env
```

A variável `CHANNEL` define por onde você conversa com o assistente. **Só um canal fica ativo por vez** e o padrão é `whatsapp`, então instalações existentes continuam funcionando.

| `CHANNEL` | Containers que sobem | `COMPOSE_PROFILES` |
| --- | --- | --- |
| `whatsapp` (padrão) | postgres, redis, evolution, app | `whatsapp` |
| `telegram` | postgres, app | _(vazio)_ |

Variáveis comuns aos dois canais:

| Variável | Descrição |
| --- | --- |
| `GEMINI_API_KEY` | Chave da API do Google Gemini — obtenha em [aistudio.google.com](https://aistudio.google.com/app/apikey) |
| `CHANNEL` | `whatsapp` (padrão) ou `telegram` |
| `COMPOSE_PROFILES` | `whatsapp` para subir Evolution API + Redis; vazio para Telegram |
| `ADMIN_SECRET` | Senha para o endpoint `/admin/qrcode` (somente WhatsApp) — defina um valor forte em produção |

#### Canal WhatsApp (`CHANNEL=whatsapp`)

```env
CHANNEL=whatsapp
COMPOSE_PROFILES=whatsapp
EVOLUTION_API_KEY=uma-chave-qualquer-para-proteger-a-api
EVOLUTION_INSTANCE=Financial Assistant
OWNER_PHONE=5511999999999
# Opcional — use se o Evolution API entregar seu número em outro formato (bug conhecido)
ALLOWED_NUMBERS=
```

| Variável | Descrição |
| --- | --- |
| `EVOLUTION_API_KEY` | Chave para proteger a sua instância da Evolution API — pode ser qualquer valor |
| `EVOLUTION_INSTANCE` | Nome da instância no Evolution API |
| `OWNER_PHONE` | Seu número de WhatsApp com código do país e DDD, sem `+` ou espaços (ex: `5511999999999`) |
| `ALLOWED_NUMBERS` | Opcional — número alternativo caso o Evolution API entregue seu número em formato diferente |

#### Canal Telegram (`CHANNEL=telegram`)

1. No Telegram, converse com o [@BotFather](https://t.me/BotFather), envie `/newbot` e siga as instruções. Ele devolve o **token** do bot.
2. Converse com o [@userinfobot](https://t.me/userinfobot) para descobrir o **seu ID numérico**.
3. Preencha o `.env`:

```env
CHANNEL=telegram
COMPOSE_PROFILES=
TELEGRAM_BOT_TOKEN=123456:token-do-botfather
TELEGRAM_CHAT_ID=987654321
```

| Variável | Descrição |
| --- | --- |
| `TELEGRAM_BOT_TOKEN` | Token do bot criado no @BotFather. Trate como senha: quem tem o token controla o bot |
| `TELEGRAM_CHAT_ID` | Seu ID numérico no Telegram. **Só esse usuário é atendido**, em conversa privada; mensagens de qualquer outra pessoa ou grupo são ignoradas |

O bot usa _long polling_: não precisa de URL pública, domínio ou HTTPS, e não expõe nenhuma porta além do `/health`.

### 3. Suba o projeto

```bash
docker compose up -d --build && docker compose logs -f app
```

- **WhatsApp:** na primeira execução, o assistente cria a instância no Evolution API e exibe um **QR code nos logs do app**.
- **Telegram:** o log mostra `canal Telegram ativo` com o `@username` do bot. Abra a conversa com ele e envie `/start` ou sua primeira mensagem.

### 4. Conecte o WhatsApp (somente canal WhatsApp)

Escaneie o QR code exibido no terminal com o seu WhatsApp:

> WhatsApp → **Aparelhos conectados** → **Conectar um aparelho** → escaneie o QR code

Após escanear, o assistente estará pronto para uso.

> Em execuções futuras, se o WhatsApp já estiver conectado, o QR code não será exibido.

### Atualizando uma instalação existente

Os containers Evolution API e Redis agora só sobem com o perfil `whatsapp`. Se você já usava o WhatsApp, adicione ao seu `.env`:

```env
COMPOSE_PROFILES=whatsapp
```

Sem isso, `docker compose up` não sobe a Evolution API e o app ficará aguardando por ela. Além disso, as portas do Postgres e do Redis passaram a escutar apenas em `127.0.0.1`.

## Uso

Com o container rodando, envie mensagens para **si mesmo** no WhatsApp ou para o seu bot no Telegram. Os exemplos abaixo valem para os dois canais.

### Registrar gasto simples
```
gastei 45 reais no almoço no pix
```

### Registrar compra parcelada
```
comprei um tênis de 300 reais em 3x no cartão
```

### Registrar despesa recorrente
```
netflix 55 reais todo mês todo dia 15
```

### Cancelar recorrente
```
cancelar netflix
```

### Registrar entrada (salário, renda)
```
recebi 6000 reais de salário
entrou 500 reais de freela no pix
```

### Registrar transferência entre contas próprias
```
coloquei 2000 reais no cofrinho
resgatei 500 do CDB
```
Transferências não afetam despesas nem entradas — servem apenas para rastrear movimentações entre suas próprias contas.

### Consultar resumo do mês
```
quanto gastei esse mês?
quanto gastei em fevereiro?
```

O resumo mostra:
- **Despesas** por categoria
- **Entradas** totais (se houver)
- **Resultado** do mês (entradas − despesas)
- **Investimentos no mês**: quanto foi aplicado, quanto foi resgatado
- **Em conta**: resultado descontando o líquido que ficou investido

### Importar extrato bancário (PDF)

Envie o PDF do extrato do seu banco diretamente no WhatsApp. O assistente processa todas as transações automaticamente:

- Despesas, entradas e transferências são classificadas pela IA
- Aplicações no cofrinho e resgates de CDB são detectados como **Transferência** — não inflam as despesas
- Transações já existentes no banco são sinalizadas para confirmação individual
- Suporta o formato de extrato do **Itaú** (e outros formatos com datas DD/MM/AAAA ou AAAA-MM-DD)

### Exportar planilha CSV

Peça ao assistente para exportar os gastos de um mês e ele enviará um arquivo `.csv` diretamente na conversa — pronto para abrir no Excel ou Google Sheets:

```
exportar meus gastos de março
me manda o csv de fevereiro 2024
quero a planilha de janeiro
exportar
```

- Se nenhum mês for especificado, exporta o **mês atual**.
- Se não houver lançamentos no período, o assistente avisa por texto.
- O arquivo vem com **BOM UTF-8** para compatibilidade com Excel.
- Colunas: Data, Descrição, Categoria, Forma de Pagamento, Tipo, Parcela, Valor (R$).
- Linhas de totais ao final: **TOTAL DESPESAS**, **TOTAL ENTRADAS** (se houver), **SALDO**, **TOTAL APLICADO** / **TOTAL RESGATADO** (se houver transferências).
- A mensagem que acompanha o arquivo já traz o resumo financeiro: despesas, entradas, resultado, aplicado/resgatado e valor em conta.

> O Gemini interpreta a intenção de exportação, então frases naturais como _"quero ver meus gastos em planilha"_ ou _"gera um csv pra mim"_ também funcionam.

### Relatório mensal automático

No primeiro dia de cada mês, o assistente envia automaticamente a planilha CSV com todos os gastos do mês anterior — sem você precisar pedir.

### Enviar recibo ou nota fiscal
Tire uma foto ou encaminhe a imagem do recibo diretamente na conversa. No Telegram, imagens enviadas "como arquivo" também são tratadas como recibo.

## Segurança e gerenciamento remoto

### Gerar QR Code para conectar o WhatsApp (somente canal WhatsApp)

Abra no navegador substituindo pelo IP da sua VPS ou `localhost` se estiver rodando localmente:

```
http://<IP-DA-VPS-OU-LOCALHOST>:3000/admin/qrcode?token=sua-senha
```

Se o WhatsApp já estiver conectado, exibe uma mensagem de confirmação. Se não, exibe o QR code para escanear — a página atualiza automaticamente a cada 30 segundos.

**Proteções implementadas (WhatsApp):**
- Requer `ADMIN_SECRET` configurado (retorna `503` se vazio)
- Rate limit de 10 requisições por minuto por IP
- `/webhook` aceita conexões apenas do container Evolution API (verificação por IP via DNS interno do Docker)

**Proteções implementadas (Telegram):**
- Apenas o `TELEGRAM_CHAT_ID` configurado, em conversa privada, é atendido; o restante é descartado antes de qualquer processamento
- O token nunca é registrado em logs: erros de rede do cliente são sanitizados
- Sem webhook: o bot busca as mensagens (long polling), então nenhuma rota pública é necessária
- Downloads de anexos limitados a 20 MB (limite da própria Bot API)

## Comandos úteis

```bash
# Subir e acompanhar logs da aplicação
docker compose up -d --build && docker compose logs -f app

# Ver logs em tempo real
docker compose logs -f app

# Parar os containers
docker compose down

# Parar e apagar todos os dados (banco, volumes)
docker compose down -v

# Acessar o banco de dados
docker compose exec postgres psql -U finassist -d finassist
```

## Estrutura do projeto

```
cmd/                                        entrypoint da aplicação
internal/
    config/                               carregamento de variáveis de ambiente
    chat/                                 lógica de conversa independente de canal
    domain/                               entidades e regras de negócio
    usecase/                              casos de uso (análise, recorrentes, consulta, exportação)
    infra/
        db/                               repositório PostgreSQL
        evolution/                        cliente da Evolution API (WhatsApp)
        gemini/                           cliente do Google Gemini
        http/                             servidor HTTP e adapter do webhook do WhatsApp
        telegram/                         cliente da Bot API e bot (long polling)
migrations/                               scripts SQL de criação do banco
```

## Licença

MIT
