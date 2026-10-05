# Go Financial Assistant

Assistente financeiro pessoal via **WhatsApp ou Telegram** (você escolhe o canal). Envie mensagens de texto, fotos de recibos ou extratos bancários em PDF para registrar despesas, entradas e transferências automaticamente. O assistente utiliza IA (Google Gemini, opcional) para interpretar as transações e armazená-las em um banco de dados PostgreSQL. A instalação é guiada pelo navegador: `make init`, `docker compose up -d` e o resto na tela.

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

- [Docker](https://www.docker.com/) com Docker Compose (e `make` e `openssl`, para o `make init`)
- Conta no Telegram (ou no WhatsApp, configurado pelo `.env`)
- Opcional: chave de API do [Google Gemini](https://aistudio.google.com/app/apikey), para entender mensagens livres, recibos e extratos

## Configuração

### Começo rápido (3 comandos)

```bash
git clone https://github.com/MarcosAAlbanoJunior/go-financial-assistant.git && cd go-financial-assistant
make init
docker compose up -d
```

Abra **http://localhost:8080** e cole o token que o `make init` gravou no `.env` (linha `SETUP_TOKEN`). O resto é no navegador, sem editar o `.env` e sem reiniciar nada.

O `make init` nunca sobrescreve o que já existe: cria o `.env` a partir do `.env.example`, a chave mestra em `secrets/app_secret_key` (preenchendo `APP_SECRET_KEY_FILE`) e um `SETUP_TOKEN` aleatório de 32 caracteres. Sem o `make`, faça à mão:

```bash
cp .env.example .env
mkdir -p secrets && (umask 077 && openssl rand -base64 32 > secrets/app_secret_key)
# no .env: APP_SECRET_KEY_FILE=/run/secrets/app_secret_key
# no .env: SETUP_TOKEN=<o resultado de: openssl rand -hex 16>
```

### O setup no navegador

Uma tela por passo, com o que fazer fora do app em lista numerada. Cada passo é salvo ao avançar: fechar no meio e voltar depois retoma do mesmo ponto (pedindo o token de novo).

1. **Token:** prova que o servidor é seu. O token só existe no `.env`; colado num campo (nunca na URL), com limite de 5 tentativas por minuto.
2. **Senha do dashboard** (mínimo de 12 caracteres), guardada só como hash argon2id. Ela só passa a valer quando o setup termina.
3. **Telegram:**
   - crie o bot no [@BotFather](https://t.me/BotFather) e cole o token (o app confere na hora);
   - abra o seu bot pelo botão da tela e mande `/start`: o app descobre a sua conta sozinho, sem `@userinfobot`, e pergunta se é você (se outra pessoa mandar antes, "não sou eu" descarta);
   - digite o código de 6 dígitos que chega no chat.
4. **Pronto:** o bot já responde (sem reiniciar) e você entra direto no dashboard. A tela mostra os próximos passos opcionais, cada um levando ao grupo certo das Configurações: Gemini, Open Finance, seus nomes nos extratos e o resumo semanal.

Depois do setup, o `SETUP_TOKEN` pode ser apagado do `.env` (ou ficar: ele não abre mais nada) e o login pede a senha e um código no chat.

**Sem o Gemini** o app funciona: o chat atende os comandos (`/saldos`, `/resumo`, `/sync`), o dashboard funciona inteiro e o que precisa de IA (mensagens livres, recibos, extratos, Coach) explica como ligar. Colocar a chave em Configurações → IA vale na hora.

### Recuperar o acesso

- **Perdeu a senha ou o chat:** coloque `SETUP_REOPEN=true` no `.env` e reinicie (`docker compose up -d`). O setup reabre com o mesmo `SETUP_TOKEN` para redefinir a senha e/ou o canal (manter o de antes ou ligar outro bot), sem mexer nos dados. Ao terminar, remova `SETUP_REOPEN` (enquanto estiver lá, nada mais acontece).
- **Emergência com o canal fora do ar:** `DASHBOARD_2FA=off` faz o login pedir só a senha. Com o canal configurado e fora do ar (token errado, Telegram fora), o login responde "não foi possível enviar o código" em vez de aceitar só a senha, e o app tenta ligar o canal de novo sozinho (a cada 5 s, até 5 min entre tentativas).

### Alternativa: tudo pelo `.env`

Quem prefere (ou já tinha) o `.env` completo não vê o setup: com a senha (`DASHBOARD_PASSWORD`) e o canal no ambiente, o app sobe direto no login, como antes. O valor salvo pelo setup ou pela página Configurações vale mais que o do ambiente ([ADR 0004](docs/decisoes/0004-precedencia-das-configuracoes.md)).

A variável `CHANNEL` define por onde você conversa com o assistente. **Só um canal fica ativo por vez.**

| `CHANNEL` | Containers que sobem | `COMPOSE_PROFILES` |
| --- | --- | --- |
| `telegram` | postgres, app, web | _(vazio)_ |
| `whatsapp` (padrão sem `CHANNEL`) | postgres, redis, evolution, app, web | `whatsapp` |

Variáveis comuns aos dois canais (todas opcionais; também se configuram na página Configurações):

| Variável | Descrição |
| --- | --- |
| `DASHBOARD_PASSWORD` | Senha do dashboard (mínimo de 12 caracteres), no lugar da definida pelo setup |
| `GEMINI_API_KEY` | Chave da API do Google Gemini, obtida em [aistudio.google.com](https://aistudio.google.com/app/apikey). Sem ela, o que precisa de IA fica desligado com aviso |
| `GEMINI_PAID_PLAN` | Padrão `false`. Declare `true` só se o projeto da chave tiver faturamento (serviços pagos): é o que libera o **Coach com IA**, que envia dados financeiros ao Gemini (veja *Coach com IA*) |
| `DIGEST_ENABLED`, `DIGEST_WEEKDAY`, `DIGEST_HOUR`, `DIGEST_TIMEZONE` | **Resumo semanal** no canal de conversa (padrão: ligado, segunda às 9h, fuso `America/Sao_Paulo`): veja *Resumo semanal*. `DIGEST_ENABLED=false` desliga |
| `COACH_GEMINI_MODEL` | Troca o modelo do Coach (padrão `gemini-3.5-flash-lite`) |
| `CHANNEL` | `telegram` ou `whatsapp` |
| `COMPOSE_PROFILES` | `whatsapp` para subir Evolution API + Redis; vazio para Telegram |
| `ADMIN_SECRET` | Senha para o endpoint `/admin/qrcode` (somente WhatsApp); defina um valor forte em produção |

O que fica sempre no ambiente: `DATABASE_URL`, a chave mestra (`APP_SECRET_KEY_FILE` ou `APP_SECRET_KEY`), `SETUP_TOKEN`/`SETUP_REOPEN`, `PORT`/`WEB_PORT`, `DASHBOARD_2FA` e as variáveis do WhatsApp.

#### Telegram (pelo .env)

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

#### WhatsApp (pelo .env)

Nesta versão o WhatsApp não entra no setup pelo navegador: configure pelo `.env` (o setup só pede a senha e mantém o canal).

```env
CHANNEL=whatsapp
COMPOSE_PROFILES=whatsapp
EVOLUTION_API_KEY=uma-chave-qualquer-para-proteger-a-api
EVOLUTION_INSTANCE=Financial Assistant
OWNER_PHONE=5511999999999
# Opcional: use se a Evolution API entregar seu número em outro formato (bug conhecido)
ALLOWED_NUMBERS=
```

| Variável | Descrição |
| --- | --- |
| `EVOLUTION_API_KEY` | Chave para proteger a sua instância da Evolution API (pode ser qualquer valor) |
| `EVOLUTION_INSTANCE` | Nome da instância na Evolution API |
| `OWNER_PHONE` | Seu número de WhatsApp com código do país e DDD, sem `+` ou espaços (ex: `5511999999999`) |
| `ALLOWED_NUMBERS` | Opcional: número alternativo caso a Evolution API entregue seu número em formato diferente |

Suba com `docker compose up -d --build && docker compose logs -f app`. Na primeira execução, o assistente cria a instância na Evolution API e exibe um **QR code nos logs do app**. Escaneie com o seu WhatsApp:

> WhatsApp → **Aparelhos conectados** → **Conectar um aparelho** → escaneie o QR code

> Em execuções futuras, se o WhatsApp já estiver conectado, o QR code não será exibido.

### Atualizando uma instalação existente

Nada muda para quem já tem o `.env` completo (senha e canal): não aparece setup e o login é o de antes. O `GEMINI_API_KEY` deixou de ser obrigatório.

Os containers Evolution API e Redis só sobem com o perfil `whatsapp`. Se você usa o WhatsApp, mantenha no `.env`:

```env
COMPOSE_PROFILES=whatsapp
```

Sem isso, `docker compose up` não sobe a Evolution API e o app ficará aguardando por ela. As portas do Postgres e do Redis escutam apenas em `127.0.0.1`.

**Migrations novas:** o Postgres só roda os scripts de `backend/migrations/` na primeira criação do banco. Em um banco existente, aplique à mão os que faltam, por exemplo a `010` (sugestões dispensadas da tela Revisão) a `011` (metas), a `012` (histórico do Coach), a `013` (decisões "cancelei" da Revisão) a `014` (regras de categoria), a `016` e a `017` (configurações salvas no dashboard e o histórico delas), a `018` (data de atualização dos dados do banco), a `019` (setup pelo navegador e recuperação com `SETUP_REOPEN`) e a `015` (bancos, logos e dados dos cartões; depois, rode uma sincronização):

```bash
docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/010_create_review_dismissals.sql
docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/011_create_goals.sql
docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/012_create_coach_analyses.sql
docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/013_create_review_decisions.sql
docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/014_create_category_rules.sql
docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/015_create_institutions.sql
docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/016_create_settings.sql
docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/017_create_settings_audit.sql
docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/018_institution_source_updated_at.sql
docker compose exec -T postgres psql -U finassist -d finassist < backend/migrations/019_create_dashboard_owner.sql
```

Sem a `019` o app sobe normalmente com o `.env` completo (só avisa no log); ela é necessária para usar o setup pelo navegador ou o `SETUP_REOPEN`.

### Open Finance (opcional)

Em vez de registrar tudo à mão, você pode conectar seus bancos pelo **Open Finance** usando o [Meu Pluggy](https://meu.pluggy.ai) (gratuito para uso pessoal, até 5 conexões do mesmo titular). O assistente passa a importar sozinho suas transações; o registro manual **continua funcionando** e é o caminho para dinheiro vivo ou para quem prefere não conectar o banco.

1. Crie sua conta em [meu.pluggy.ai](https://meu.pluggy.ai) e conecte seus bancos (você autoriza o compartilhamento no app do próprio banco).
2. Crie uma conta no [Dashboard do Pluggy](https://dashboard.pluggy.ai) e uma aplicação de desenvolvimento. Ela fornece o **Client ID** e o **Client Secret**.
3. No Dashboard, habilite o conector **MeuPluggy** e, na aplicação **Demo**, vincule o seu Meu Pluggy via OAuth. Faça isso **uma vez por banco** (não por conta).
4. Ainda na Demo, no menu de três pontos de cada conexão, copie o **itemId**.
5. Preencha o `.env` e reinicie:

```env
PLUGGY_CLIENT_ID=seu-client-id
PLUGGY_CLIENT_SECRET=seu-client-secret
PLUGGY_ITEM_IDS=itemid-do-banco-1,itemid-do-banco-2
```

| Variável | Descrição |
| --- | --- |
| `PLUGGY_CLIENT_ID` / `PLUGGY_CLIENT_SECRET` | Credenciais da aplicação no Dashboard. O app gera e renova a `apiKey` sozinho (ela expira em 2 horas) |
| `PLUGGY_ITEM_IDS` | `itemId` de cada banco conectado, separados por vírgula. Precisam ser UUIDs |
| `SYNC_INTERVAL_HOURS` | Intervalo entre sincronizações (padrão `6`) |
| `SYNC_LOOKBACK_DAYS` | Quantos dias para trás buscar a cada sincronização (padrão `60`, máx. `365`) |

Se algum `PLUGGY_*` estiver preenchido, os três são obrigatórios. Sem nenhum, o Open Finance fica desligado.

**Como funciona**
- Sincroniza ao subir e a cada `SYNC_INTERVAL_HOURS`. Para forçar agora, envie `/sync` (Telegram) ou `sincronizar` (qualquer canal).
- É **idempotente**: cada transação tem o ID de origem gravado, então sincronizar de novo não duplica nada.
- **Conciliação com o manual:** se você registrou "gastei 45 no almoço" e depois o Pix chega pelo banco, o assistente liga os dois em vez de contar duas vezes (mesmo tipo e valor, data com até 3 dias de diferença; recorrentes casam no mesmo mês).
- **Sem dupla contagem no cartão:** pagamento de fatura e transferência entre contas do mesmo titular são ignorados, porque as compras do cartão já entram uma a uma. Aplicações e resgates de investimento viram **Transferência**.
- Compras parceladas no cartão chegam parcela a parcela, com a descrição `(2/3)`.
- **Classificação:** a categoria vem do Pluggy; quando ele não classifica a despesa, palavras da descrição decidem (iFood, posto, farmácia, assinaturas...). Pix para pessoas e o que não for óbvio ficam em "Outros". As regras estão em `backend/internal/infra/pluggy/mapper.go`.
- **Aplicação automática do Itaú** ("APLIC AUT MAIS") é ignorada: é só o banco varrendo o saldo da conta para um CDB e de volta.
- **Histórico:** a primeira sincronização busca `SYNC_LOOKBACK_DAYS` dias (padrão 60). Para trazer o ano todo (o Pluggy guarda 12 meses), use `SYNC_LOOKBACK_DAYS=365` uma vez e envie `/sync`; é idempotente e pode voltar ao padrão depois.
- **Histórico estimado:** o Pluggy não informa saldos passados, então os meses anteriores à primeira sincronização são **estimados** pelas movimentações de cada posição (saldo do primeiro registro menos o aplicado depois, sem contar rendimentos, o que os deixa um pouco acima do real) e aparecem em cinza com um aviso. Da primeira sincronização em diante o saldo é o exato. Depende de o banco entregar as movimentações; o log "investimentos sincronizados" mostra quantas vieram.
- **Investimentos:** se o banco conectado tiver posições (CDB, fundos, ações, previdência...), o app guarda o saldo líquido de cada uma a cada sincronização (tabela `investments`, mais o saldo diário em `investment_balances`). O Pluggy não informa saldos passados, então o histórico do patrimônio começa na primeira sincronização. Titular, CNPJ do emissor e número da posição são descartados.
- **Transferências entre contas suas:** defina `OWN_NAMES` (seus nomes como aparecem no extrato, separados por vírgula) e Pix, TED e DOC com esse nome são ignorados nas próximas sincronizações. Para limpar o que já foi gravado, faça um backup (`make backup`) e cancele os lançamentos: `UPDATE payments SET status='CANCELLED' WHERE id IN (SELECT pay.id FROM payments pay JOIN purchases p ON p.id = pay.purchase_id WHERE p.description ~* '^pix (enviado|recebido) SEU NOME *$')`.
- **Contas e cartões** são guardados na tabela `accounts` (nome, tipo, 4 últimos dígitos, saldo e limite do cartão), e cada transação aponta para a conta de origem. O app **não** guarda CPF, nome do titular nem o número completo da conta. Transações sincronizadas antes dessa tabela ganham a conta na primeira sincronização depois da atualização, desde que estejam dentro de `SYNC_LOOKBACK_DAYS`.

**Limitações**
- O Meu Pluggy atualiza os dados cerca de **uma vez por dia** e não permite forçar atualização; `/sync` só busca o que o Pluggy já tem.
- Estornos no cartão não são subtraídos das despesas.
- O saldo de um cartão é o valor que o Pluggy informa (em geral a fatura atual). Ele só atualiza quando o Pluggy atualiza.
- Transações alteradas ou removidas depois no banco não são atualizadas aqui.
- A classificação usa as categorias do Pluggy (mapeamento em `internal/infra/pluggy/mapper.go`); o que não for reconhecido vira "Outros".

## Dashboard (front-end)

O front-end (React + TypeScript + Vite) fica em `frontend/` e é servido por um container nginx que também repassa `/api` ao app (mesma origem, sem CORS).

1. Na primeira vez, o dashboard abre o **setup** (veja *Configuração*): senha e canal definidos no navegador. Quem tem `DASHBOARD_PASSWORD` e o canal no `.env` vai direto ao login.
2. `docker compose up -d --build` e abra **http://localhost:8080** (mude a porta com `WEB_PORT`).
3. Entre com a senha e o **código que chega no seu chat** (segundo fator; `DASHBOARD_2FA=off` desliga, por exemplo se você perder o acesso ao chat; para trocar a senha ou o canal, `SETUP_REOPEN`). Telas: Painel (saldos por banco e cartão), Visão geral, Gastos, Comparações, Transações, Classificar, Orçamento, Revisão, Metas, Coach, Projeção, Investimentos e **Configurações** (edição no navegador do que antes só existia no `.env`, com segredos cifrados).

Detalhes de cada tela, segurança e desenvolvimento em [docs/dashboard.md](docs/dashboard.md). O container escuta só em `127.0.0.1`; para acessar de outro dispositivo, ponha na frente um proxy com **HTTPS** (a API e o dashboard devem ficar na mesma origem).

## Backup e restauração

O serviço `backup` do Docker Compose grava um `pg_dump` completo do banco em `./backups` ao subir e depois a cada 24 horas, guardando os 14 mais recentes (`BACKUP_INTERVAL_HOURS` e `BACKUP_KEEP`). Sem isso, perder o volume do Postgres (um `docker compose down -v`, um disco com problema) apagaria tudo: lançamentos, metas, regras, decisões e o histórico do Coach.

- **Fora do git e privado:** `./backups` está no `.gitignore` e os arquivos ficam com permissão 600. Eles contêm todos os seus dados financeiros em texto: **não os envie a serviços de terceiros sem criptografar** (por exemplo `age` ou `gpg --symmetric arquivo.dump`) e guarde também uma cópia **fora da máquina** (outro disco ou servidor seu): um backup no mesmo disco não protege de perder o disco.
- **Dono dos arquivos:** o serviço roda com o usuário `1000:1000`; se o seu for outro (`id -u`, `id -g`), defina `BACKUP_UID` e `BACKUP_GID` no `.env` e crie a pasta antes (`mkdir -p backups`).
- **Backup na hora:** `make backup`.
- **Restaurar** (substitui os dados atuais): `make restore FILE=backups/finassist-AAAAMMDD-HHMMSS.dump`. Para testar sem risco, restaure num banco à parte: `docker compose exec -T postgres psql -U finassist -d finassist -c "CREATE DATABASE teste"` e depois `docker compose exec -T postgres pg_restore -U finassist -d teste --no-owner < backups/<arquivo>.dump`.

## Resumo semanal

Toda semana (padrão: segunda às 9h, no fuso configurado) o app manda ao seu chat (Telegram ou WhatsApp) um resumo curto do que merece atenção, **calculado só por código, sem IA e sem enviar nada a terceiros além do próprio canal**: total do mês até agora, possíveis cobranças duplicadas, contas novas do mês, contas que você marcou como canceladas e **voltaram a ser cobradas**, metas de redução que estouram o teto no ritmo atual, metas de juntar que não cabem na sobra projetada e a economia já realizada. Sem nada a avisar, ele diz "Sem alertas esta semana". No Telegram, `/resumo` pede um na hora. `/saldos` mostra o mesmo painel de saldos em texto (total, bancos, cartões e vencimentos). Se o app estiver desligado na hora marcada, aquela semana é pulada. O Telegram (ou o WhatsApp) recebe os nomes das contas em texto, como em qualquer mensagem do bot.

## Coach com IA (opcional)

A tela **Coach** pede ao Google Gemini que interprete a Revisão e as Metas e priorize cortes. Só sugere e pergunta; os números vêm sempre do código; nunca roda em segundo plano. Exige `GEMINI_PAID_PLAN=true` (ou ligar na página Configurações) por causa dos termos do plano gratuito do Google. O que é enviado, o histórico e os limites estão em [docs/coach.md](docs/coach.md).

## API do dashboard

O app expõe uma API JSON sob `/api` (sessão por cookie assinado, escritas só com JSON na mesma origem). Enquanto o app não está configurado, ela só atende o setup (`/api/setup/*`) e o resto responde `409 {"setup": "required"}`. Todas as rotas, autenticação e segurança estão em [docs/api.md](docs/api.md).

## Uso

Envie mensagens para **si mesmo** no WhatsApp ou para o seu bot no Telegram: gastos simples, compras parceladas, despesas recorrentes, entradas, transferências, consulta do mês, extrato em PDF, planilha CSV, recibos e comandos (`/sync`, `/resumo`, `/saldos`). Exemplos de cada um em [docs/uso-do-chat.md](docs/uso-do-chat.md).

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

**Proteções implementadas (setup pelo navegador):**
- Só abre com o `SETUP_TOKEN` do `.env` (mínimo de 24 caracteres), colado num campo, comparado em tempo constante e com 5 tentativas por minuto por IP; a sessão do setup (`fa_setup`) é separada da do dashboard e não dá acesso a mais nada
- Só a conta que mandou `/start` **e** digitou o código enviado a ela vira dona do bot; nada do que foi digitado num passo incompleto vale como configuração
- Concluído, as rotas do setup respondem `404`; cada passo vai para o histórico de alterações (sem valores). Detalhes na [ADR 0008](docs/decisoes/0008-setup-pelo-navegador.md)

**Proteções implementadas (Telegram):**
- Apenas o `TELEGRAM_CHAT_ID` configurado, em conversa privada, é atendido; o restante é descartado antes de qualquer processamento
- O token nunca é registrado em logs: erros de rede do cliente são sanitizados
- Sem webhook: o bot busca as mensagens (long polling), então nenhuma rota pública é necessária
- Downloads de anexos limitados a 20 MB (limite da própria Bot API)

## Comandos úteis

```bash
# Preparar uma instalação nova (.env, chave mestra e SETUP_TOKEN, sem sobrescrever nada)
make init

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
backend/                                    aplicação em Go
    cmd/                                    entrypoint da aplicação
    internal/
        app/                                composição: monta e liga todas as peças
        config/                             leitura e validação do ambiente
        chat/                               lógica de conversa independente de canal
        domain/                             entidades e regras puras; domain/ports: só interfaces
        usecase/                            casos de uso por contexto: ledger, openfinance, planning, review, coach, balances, insights
        settings/                           configurações do dashboard (valor salvo > ambiente > padrão), segredos cifrados
        setup/                              estado do setup pelo navegador e a regra de "configurado"
        auth/                               segundo fator (códigos no chat) e hash argon2id da senha
        infra/
            db/                             repositório PostgreSQL
            evolution/                      cliente da Evolution API (WhatsApp)
            gemini/                         cliente do Google Gemini
            http/                           servidor HTTP e adapter do webhook do WhatsApp
            pluggy/                         cliente do Open Finance (Meu Pluggy)
            telegram/                       cliente da Bot API e bot (long polling)
    migrations/                             scripts SQL de criação do banco
frontend/                                   dashboard em React + TypeScript (Vite) e nginx
    src/api/                                cliente da API e hooks de consulta
    src/lib/                                formatação, meses e tema (com testes)
    src/components/, src/pages/             telas e componentes
docker-compose.yml, Makefile, .env.example  na raiz
docs/                                   arquitetura, regras de cálculo, decisões, API, telas e uso do chat
```

Camadas e fluxos em [docs/arquitetura.md](docs/arquitetura.md); como contribuir em [CONTRIBUTING.md](CONTRIBUTING.md).

## Licença

MIT
