# Go Financial Assistant

Assistente financeiro pessoal via WhatsApp. Envie mensagens de texto ou fotos de recibos e comprovantes para registrar despesas automaticamente. O assistente utiliza IA (Google Gemini) para interpretar os gastos e armazená-los em um banco de dados PostgreSQL.

## Como funciona

Você envia uma mensagem para **si mesmo** no WhatsApp — texto descrevendo um gasto ou uma foto de recibo/nota fiscal — e o assistente registra a despesa automaticamente.

**Exemplos de mensagens:**
- `"gastei 45 reais no almoço no pix"`
- `"netflix 55 reais todo mês todo dia 15"`
- `"cancelar netflix"`
- `"quanto gastei em março?"`
- `"exportar meus gastos de março"`
- Foto de um recibo ou nota fiscal

O assistente interpreta o tipo de gasto (único, parcelado, recorrente), categoria, valor e forma de pagamento.

## Tecnologias

- **Go** — aplicação principal
- **Google Gemini** — análise e interpretação das mensagens
- **Evolution API** — integração com WhatsApp
- **PostgreSQL** — armazenamento das despesas
- **Redis** — cache da Evolution API
- **Docker / Docker Compose** — infraestrutura

## Pré-requisitos

- [Docker](https://www.docker.com/) com Docker Compose
- Chave de API do [Google Gemini](https://aistudio.google.com/app/apikey)
- Conta no WhatsApp

## Configuração

### 1. Clone o repositório

```bash
git clone https://github.com/MarcosAAlbanoJunior/go-financial-assistant.git
cd go-financial-assistant
```

### 2. Configure as variáveis de ambiente

Copie o arquivo de exemplo e preencha com suas informações:

```bash
cp .env.example .env
```

Edite o `.env`:

```env
PORT=3000
DATABASE_URL=postgres://finassist:finassist@localhost:5432/finassist?sslmode=disable
GEMINI_API_KEY=sua-chave-do-gemini

EVOLUTION_API_KEY=uma-chave-qualquer-para-proteger-a-api
EVOLUTION_API_URL=http://localhost:8082
EVOLUTION_INSTANCE=Financial Assistant
OWNER_PHONE=5511999999999

# Opcional — adicione aqui se o Evolution API usar um numero diferente do seu número (bug conhecido)
ALLOWED_NUMBERS=

# Senha para o endpoint /admin/qrcode (obrigatório para usar o endpoint em produção)
ADMIN_SECRET=sua-senha-segura
```

| Variável | Descrição |
| --- | --- |
| `GEMINI_API_KEY` | Chave da API do Google Gemini — obtenha em [aistudio.google.com](https://aistudio.google.com/app/apikey) |
| `EVOLUTION_API_KEY` | Chave para proteger a sua instância da Evolution API — pode ser qualquer valor |
| `EVOLUTION_INSTANCE` | Nome da instância no Evolution API |
| `OWNER_PHONE` | Seu número de WhatsApp com código do país e DDD, sem `+` ou espaços (ex: `5511999999999`) |
| `ALLOWED_NUMBERS` | Opcional — número alternativo caso o Evolution API entregue seu número em formato diferente |
| `ADMIN_SECRET` | Senha para acessar o endpoint `/admin/qrcode` — defina um valor forte em produção |

### 3. Suba o projeto

```bash
docker compose up -d --build && docker compose logs -f app
```

Na primeira execução, o assistente irá criar a instância no Evolution API automaticamente e exibir um **QR code no terminal**.

Basicamente o QR code vem dos logs do app em Go

### 4. Conecte o WhatsApp

Escaneie o QR code exibido no terminal com o seu WhatsApp:

> WhatsApp → **Aparelhos conectados** → **Conectar um aparelho** → escaneie o QR code

Após escanear, o assistente estará pronto para uso.

> Em execuções futuras, se o WhatsApp já estiver conectado, o QR code não será exibido.

## Uso

Com o container rodando e o WhatsApp conectado, envie mensagens para **si mesmo** no WhatsApp.

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

### Consultar resumo do mês
```
quanto gastei esse mês?
quanto gastei em fevereiro?
```

### Exportar planilha CSV

Peça ao assistente para exportar os gastos de um mês e ele enviará um arquivo `.csv` diretamente no WhatsApp — pronto para abrir no Excel ou Google Sheets:

```
exportar meus gastos de março
me manda o csv de fevereiro 2024
quero a planilha de janeiro
exportar
```

- Se nenhum mês for especificado, exporta o **mês atual**.
- Se não houver gastos no período, o assistente avisa por texto.
- O arquivo vem com **BOM UTF-8** para compatibilidade com Excel.
- Colunas: Data, Descrição, Categoria, Forma de Pagamento, Tipo, Parcela, Valor (R$).

> O Gemini interpreta a intenção de exportação, então frases naturais como _"quero ver meus gastos em planilha"_ ou _"gera um csv pra mim"_ também funcionam.

### Relatório mensal automático

No primeiro dia de cada mês, o assistente envia automaticamente a planilha CSV com todos os gastos do mês anterior — sem você precisar pedir.

### Enviar recibo ou nota fiscal
Tire uma foto ou encaminhe a imagem do recibo diretamente no WhatsApp.

## Segurança e gerenciamento remoto

### Gerar QR Code para conectar o WhatsApp

Abra no navegador substituindo pelo IP da sua VPS ou `localhost` se estiver rodando localmente:

```
http://<IP-DA-VPS-OU-LOCALHOST>:3000/admin/qrcode?token=sua-senha
```

Se o WhatsApp já estiver conectado, exibe uma mensagem de confirmação. Se não, exibe o QR code para escanear — a página atualiza automaticamente a cada 30 segundos.

**Proteções implementadas:**
- Requer `ADMIN_SECRET` configurado (retorna `503` se vazio)
- Rate limit de 10 requisições por minuto por IP
- `/webhook` aceita conexões apenas do container Evolution API (verificação por IP via DNS interno do Docker)

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
    domain/                               entidades e regras de negócio
    usecase/                              casos de uso (análise, recorrentes, consulta, exportação)
    infra/
        db/                               repositório PostgreSQL
        evolution/                        cliente da Evolution API (WhatsApp)
        gemini/                           cliente do Google Gemini
        http/                             servidor HTTP e webhook handler
migrations/                               scripts SQL de criação do banco
```

## Licença

MIT
