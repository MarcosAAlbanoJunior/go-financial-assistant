# FinAssist — seu assistente financeiro pessoal

Conecte seus bancos pelo **Open Finance**, converse com o seu próprio **bot no Telegram** e acompanhe tudo num **dashboard** no navegador. Roda no seu computador ou servidor, com os seus dados no seu banco de dados.

```
  Seus bancos ──► Meu Pluggy (Open Finance) ──┐
                                              ├──► FinAssist ──► Dashboard (navegador)
  Você no Telegram ("gastei 45 no almoço") ───┘          └────► Bot no Telegram (saldos, resumo, avisos)
```

- **Open Finance:** transações, saldos, cartões e investimentos chegam sozinhos, sem digitar nada.
- **Telegram:** registre gastos em linguagem natural, mande foto de recibo ou o PDF do extrato, peça saldos e o resumo da semana.
- **Dashboard:** painel de saldos, gastos por categoria, orçamento, metas, revisão de gastos com sugestões de corte, projeção e investimentos.
- **Instalação guiada:** três comandos e o resto é feito no navegador.

## Sumário

1. [O que dá para fazer](#o-que-dá-para-fazer)
2. [Instalação](#instalação)
3. [Passo 1: o setup no navegador (senha e Telegram)](#passo-1-o-setup-no-navegador-senha-e-telegram)
4. [Passo 2: conectar os seus bancos (Open Finance)](#passo-2-conectar-os-seus-bancos-open-finance)
5. [Passo 3: ligar a IA (opcional)](#passo-3-ligar-a-ia-opcional)
6. [Passo 4: ajustes finos](#passo-4-ajustes-finos)
7. [Usando no dia a dia](#usando-no-dia-a-dia)
8. [A página Configurações](#a-página-configurações)
9. [Segurança](#segurança)
10. [Recuperar o acesso](#recuperar-o-acesso)
11. [Backup e restauração](#backup-e-restauração)
12. [Atualizando uma instalação existente](#atualizando-uma-instalação-existente)
13. [Comandos úteis e documentação](#comandos-úteis-e-documentação)

## O que dá para fazer

| No dashboard | O que mostra |
| --- | --- |
| **Painel** (página inicial) | Total em conta, cada banco com a conta corrente e os cartões (fatura, limite usado, vencimento) e alertas de limite e vencimento |
| **Visão geral** | Receitas, despesas e saldo do mês, comparação com o mês anterior e os últimos 12 meses |
| **Transações** | Tudo o que entrou e saiu, por categoria, por dia ou em lista, com filtros e busca |
| **Gastos** | Despesas do mês por categoria, forma de pagamento e conta |
| **Classificar** | As contas que ficaram em "Outros" (geralmente Pix): escolha a categoria uma vez e as próximas já chegam certas |
| **Orçamento** | Despesas em fixas, parceladas e variáveis, e quanto da renda já está comprometido |
| **Revisão** | Mapa de calor por categoria e sugestões de corte (assinaturas, gasto formiga, cobranças duplicadas, aumentos), com a economia por ano |
| **Metas** | Juntar um valor até uma data, reduzir uma categoria ou manter uma reserva de N meses |
| **Coach** | A IA comenta a revisão e as metas e sugere prioridades (opcional, veja o [Passo 3](#passo-3-ligar-a-ia-opcional)) |
| **Projeção** | Os próximos 6, 12 ou 24 meses e um simulador de financiamento |
| **Comparações** | Um mês contra o outro, por categoria |
| **Investimentos** | Patrimônio investido por tipo e produto e a evolução do saldo |

| No Telegram | O que acontece |
| --- | --- |
| `/saldos` | Painel de saldos em texto: total, bancos, cartões e vencimentos |
| `/resumo` | Resumo da semana: total do mês, cobranças duplicadas, contas novas, metas em risco |
| `/sync` | Busca agora as transações novas dos seus bancos |
| `gastei 45 reais no almoço no pix` | Registra a despesa (precisa da IA) |
| Foto de recibo ou PDF do extrato | Lê e registra as transações (precisa da IA) |
| Toda segunda às 9h | O resumo semanal chega sozinho (dia e hora configuráveis) |
| Dia 1 de cada mês | A planilha CSV do mês anterior chega sozinha |

## Instalação

**Você precisa de:** [Docker](https://docs.docker.com/get-docker/) com Docker Compose, `make` e `openssl` (já vêm na maioria dos Linux e no macOS) e uma conta no Telegram.

```bash
git clone https://github.com/MarcosAAlbanoJunior/go-financial-assistant.git && cd go-financial-assistant
make init
docker compose up -d
```

Pronto: abra **http://localhost:8080**.

O `make init` prepara o arquivo `.env` com o mínimo necessário e nunca sobrescreve nada que já exista:

```
$ make init
criado .env a partir do .env.example
criado secrets/app_secret_key (chave mestra, fora do git)
APP_SECRET_KEY_FILE=/run/secrets/app_secret_key gravada no .env
SETUP_TOKEN gerado no .env

Pronto. Agora:
  1. docker compose up -d
  2. abra http://localhost:8080
  3. cole o token do arquivo .env (linha SETUP_TOKEN)
```

<details>
<summary>Sem o <code>make</code>? Faça à mão</summary>

```bash
cp .env.example .env
mkdir -p secrets && (umask 077 && openssl rand -base64 32 > secrets/app_secret_key)
openssl rand -hex 16   # copie o resultado
```

No `.env`, preencha `APP_SECRET_KEY_FILE=/run/secrets/app_secret_key` e `SETUP_TOKEN=` com o valor copiado. Depois, `docker compose up -d`.
</details>

## Passo 1: o setup no navegador (senha e Telegram)

Ao abrir http://localhost:8080 pela primeira vez, aparece **Configurar o FinAssist**, com uma barra de progresso de 4 passos. Pode fechar no meio: tudo é salvo a cada passo e você continua de onde parou.

### 1.1 Token de setup

Abra o arquivo `.env` na pasta do projeto e copie o valor da linha `SETUP_TOKEN=` (só o que vem depois do `=`). Cole na tela.

> **Por que isso?** Só quem tem acesso ao servidor tem o token. Assim, ninguém que encontre a página antes de você consegue virar dono do app.

### 1.2 Senha do dashboard

Escolha uma senha com pelo menos 12 caracteres (uma frase com espaços é ótima). O login vai pedir essa senha **e** um código que chega no seu Telegram.

### 1.3 Criar o seu bot no Telegram

O FinAssist conversa com você por um bot que é só seu. Para criar o bot:

1. No Telegram, abra o [@BotFather](https://t.me/BotFather) (o bot oficial que cria bots) e toque em **Iniciar**.
2. Mande `/newbot`.
3. Ele pede um **nome** (aparece nas conversas, ex.: `Minhas Finanças`) e depois um **usuário** terminado em `bot` (ex.: `minhas_financas_bot`).
4. Ele responde com o **token** do bot:

```
Você:      /newbot
BotFather: Alright, a new bot. How are we going to call it? Please choose a name for your bot.
Você:      Minhas Finanças
BotFather: Good. Now let's choose a username for your bot. It must end in `bot`.
Você:      minhas_financas_bot
BotFather: Done! Congratulations on your new bot. You will find it at t.me/minhas_financas_bot.
           Use this token to access the HTTP API:
           123456789:AAH4r9xYzExemploDeTokenNaoUseEste
```

Copie o token (a linha `123456789:AA...`) e cole na tela do setup. O app confere na hora e mostra **Bot @minhas_financas_bot encontrado ✓**.

> **Trate o token como uma senha:** quem tem o token controla o bot. Ele fica guardado cifrado no banco.

### 1.4 Mandar /start (o app descobre quem é você)

Você não precisa descobrir o seu ID do Telegram. A tela mostra o botão **Abrir t.me/minhas_financas_bot**:

1. Toque no botão (no celular ou no computador) para abrir o seu bot.
2. Toque em **Iniciar** (ou mande `/start`).

Em segundos, a tela mostra:

> Recebemos a mensagem de **Maria Silva** (@maria). É você?

Responda **Sim, sou eu**. Se aparecer o nome de outra pessoa (alguém achou o seu bot antes), responda **Não, não sou eu** e mande `/start` de novo.

### 1.5 Confirmar com o código

O bot manda um código de 6 dígitos para você no Telegram:

```
🔑 Código para confirmar este chat no setup do FinAssist: 482913
Vale 5 minutos. Se não foi você que está configurando o app, ignore esta mensagem.
```

Digite o código na tela. **Pronto!** O bot já está respondendo e você entra direto no dashboard. A tela final mostra os próximos passos opcionais, cada um com um link para o lugar certo em Configurações.

> Mande `/saldos` ao bot para testar. Só a sua conta é atendida: mensagens de qualquer outra pessoa (ou de grupos) são ignoradas.

## Passo 2: conectar os seus bancos (Open Finance)

O FinAssist usa o [Meu Pluggy](https://meu.pluggy.ai), um serviço de Open Finance **gratuito para uso pessoal** (até 5 conexões do mesmo titular). Você autoriza o compartilhamento no app do seu próprio banco, como em qualquer Open Finance; o FinAssist só **lê** os dados e nunca movimenta dinheiro.

Você vai precisar de três coisas do Pluggy: **Client ID**, **Client Secret** e o **Item ID** de cada banco.

### 2.1 Conectar os bancos no Meu Pluggy

1. Crie a sua conta em [meu.pluggy.ai](https://meu.pluggy.ai).
2. Conecte cada banco: escolha o banco, e o próprio app do banco abre para você autorizar o compartilhamento.

### 2.2 Criar a aplicação no Dashboard do Pluggy (Client ID e Client Secret)

1. Crie uma conta no [Dashboard do Pluggy](https://dashboard.pluggy.ai) (é outro site, para desenvolvedores).
2. Crie uma **aplicação de desenvolvimento**. Ela mostra o **Client ID** e o **Client Secret**: guarde os dois.

### 2.3 Ligar o Meu Pluggy à aplicação (Item IDs)

1. No Dashboard, habilite o conector **MeuPluggy**.
2. Na aplicação **Demo**, vincule o seu Meu Pluggy (login com a sua conta do Meu Pluggy). Faça isso **uma vez por banco** (não por conta).
3. Ainda na Demo, no menu de três pontos de cada conexão, copie o **itemId**. Ele tem este formato: `a1b2c3d4-e5f6-7890-abcd-ef1234567890`.

### 2.4 Colar no FinAssist

No dashboard, abra **Configurações** (no fim do menu lateral) e vá ao grupo **Open Finance (Meu Pluggy)**:

| Campo | O que colar |
| --- | --- |
| Client ID | o Client ID da aplicação |
| Client Secret | o Client Secret (fica cifrado e nunca mais aparece na tela) |
| Item IDs dos bancos | os itemIds, separados por vírgula |

1. Clique em **Salvar**. Como são dados sensíveis, o app manda um código de confirmação no seu Telegram: digite-o na janela que aparece.
2. Clique em **Testar conexão**. O esperado é: *"Tudo certo: credenciais aceitas e 2 conexão(ões) encontrada(s)."*
3. Não precisa reiniciar: a primeira sincronização começa sozinha em instantes. Depois, o app sincroniza a cada 6 horas.

Para buscar na hora, use o botão **Sincronizar** do menu lateral ou mande `/sync` no Telegram.

### Bom saber

- **Uma vez por dia:** o Meu Pluggy atualiza os dados cerca de uma vez por dia e não permite forçar a atualização; "Sincronizar" busca o que o Pluggy já tem. O Painel mostra a idade real dos dados de cada banco.
- **Nada duplica:** sincronizar de novo não repete transações. Se você registrou "gastei 45 no almoço" no Telegram e o Pix chega depois pelo banco, os dois são ligados em vez de contados duas vezes.
- **Cartão sem dupla contagem:** o pagamento da fatura é ignorado, porque as compras do cartão já entram uma a uma. Parcelas caem no mês da fatura.
- **Investimentos:** aplicações e resgates viram transferências (não inflam despesas nem renda), e o saldo das posições (CDB, fundos, ações, previdência) aparece em **Investimentos**.
- **Privacidade:** o app não guarda CPF, nome do titular nem o número completo da conta.

## Passo 3: ligar a IA (opcional)

A IA (Google Gemini) entende o que você escreve no Telegram, lê fotos de recibo e PDFs de extrato, sugere categorias e alimenta o **Coach**. **Sem ela o app funciona:** o Open Finance, o dashboard inteiro e os comandos `/saldos`, `/resumo` e `/sync` não dependem da IA. Se você mandar uma mensagem livre sem a IA ligada, o bot explica como ligar.

1. Entre em [aistudio.google.com/app/apikey](https://aistudio.google.com/app/apikey) com a sua conta Google.
2. Clique em **Create API key** e copie a chave.
3. No dashboard: **Configurações → IA (Gemini)** → cole em **Chave da API do Gemini** → **Salvar** (com o código no Telegram) → **Testar conexão**.

Vale na hora, sem reiniciar. Teste mandando ao bot:

```
Você:  gastei 45 reais no almoço no pix
Bot:   ✅ Despesa registrada!
       💰 R$ 45.00
       📝 Almoço
       🏷️ Alimentação
       💳 Pix
```

**Coach e privacidade.** O Coach envia ao Gemini valores e nomes de estabelecimentos (nunca nomes de pessoas). No plano gratuito, o Google pode usar e revisar esse conteúdo, por isso o Coach só funciona se você marcar **"O projeto da chave tem faturamento (plano pago)"** no mesmo grupo. Antes de cada análise, a tela mostra exatamente o que será enviado. Detalhes em [docs/coach.md](docs/coach.md).

## Passo 4: ajustes finos

Tudo em **Configurações**, valendo na hora:

| Grupo | Ajuste | Para quê |
| --- | --- | --- |
| Sincronização | **Seus nomes nos extratos** (ex.: `MARIA DA SILVA, MARIA S SILVA`) | Pix e TED com o seu nome são entre contas suas: deixam de contar como gasto e renda. Ao salvar, o app procura os antigos e pergunta se deve desconsiderá-los |
| Sincronização | **Dias para trás** = `365` (uma vez) e depois **Sincronizar** | Traz o ano inteiro de histórico (o Pluggy guarda 12 meses). Melhora o Orçamento, a Revisão e a Projeção. Pode voltar a 60 depois |
| Resumo semanal | Dia, hora e fuso | Quando o resumo chega no Telegram |

Depois, abra **Classificar** e dê categoria às contas que ficaram em "Outros": as próximas sincronizações já chegam classificadas.

## Usando no dia a dia

### No Telegram

Além do que chega pelo banco, você pode registrar o que quiser em linguagem natural (com a IA ligada):

| Você manda | O que acontece |
| --- | --- |
| `gastei 45 reais no almoço no pix` | Despesa |
| `comprei um tênis de 300 reais em 3x no cartão` | Compra parcelada |
| `netflix 55 reais todo mês todo dia 15` | Despesa recorrente (lançada sozinha todo mês) |
| `cancelar netflix` | Para a recorrente |
| `recebi 6000 reais de salário` | Entrada |
| `coloquei 2000 no cofrinho` | Transferência entre contas suas (não conta como gasto) |
| `quanto gastei em março?` | Resumo do mês por categoria |
| `exportar meus gastos de março` | Planilha CSV na conversa |
| Foto de recibo ou nota fiscal | Lê e registra a despesa |
| PDF do extrato do banco | Importa as transações e pergunta sobre as que já existem |

Comandos que funcionam com ou sem IA: `/saldos`, `/resumo` e `/sync`. Mais exemplos em [docs/uso-do-chat.md](docs/uso-do-chat.md).

### No dashboard

Entre com a senha e o código que chega no Telegram. Cada tela está descrita em [O que dá para fazer](#o-que-dá-para-fazer) e em detalhes em [docs/dashboard.md](docs/dashboard.md). O mês e os filtros ficam na URL, os gráficos têm visão em tabela e o tema claro/escuro segue o sistema.

## A página Configurações

O que antes ficava no `.env` agora se edita no navegador. Cada campo mostra de onde vem o valor (salvo aqui, `.env` ou padrão) e tem **Voltar ao valor do ambiente ou padrão**.

| Grupo | Campos | Quando vale |
| --- | --- | --- |
| Resumo semanal | ligar/desligar, dia, hora, fuso | na hora |
| Sincronização | intervalo (horas), dias para trás, seus nomes | na hora |
| Open Finance (Meu Pluggy) | Client ID, Client Secret, Item IDs | na hora |
| IA (Gemini) | chave, plano pago (Coach), modelo do Coach | na hora |
| Telegram | token do bot, seu ID | depois de reiniciar (botão **Reiniciar agora** na página) |

- **Segredos** (Client Secret, chave do Gemini, token do bot) ficam cifrados no banco com a chave mestra, que nunca vai para o banco. Depois de salvos, nunca voltam para a tela: só aparece "Configurado".
- **Mudanças sensíveis** (segredos, bancos conectados, quem fala com o bot) pedem um código novo no Telegram. Assim, uma sessão roubada não basta.
- **Histórico de alterações** no fim da página, sem os valores, e um aviso no Telegram quando algo sensível muda.

## Segurança

- **Login em dois passos:** senha + código de 6 dígitos no Telegram (vale 5 minutos, uma vez só). Cada login avisa no chat. Ligue também a [verificação em duas etapas](https://telegram.org/faq#q-how-does-2-step-verification-work) da sua conta do Telegram.
- **Só você fala com o bot:** só a conta confirmada no setup, em conversa privada, é atendida.
- **Senha guardada com hash** (argon2id), nunca em texto.
- **Tudo em `127.0.0.1`:** o dashboard e o banco não ficam expostos na rede. Para acessar de outro dispositivo, coloque na frente um proxy com **HTTPS** (Caddy, Traefik, Cloudflare Tunnel...): sem HTTPS, a senha trafega em claro.
- **O bot não precisa de porta aberta:** ele busca as mensagens no Telegram (long polling), sem webhook nem domínio.
- **O que fica no `.env`:** só o que não pode morar no banco: a chave mestra, o `SETUP_TOKEN`, portas e `DASHBOARD_2FA`. Mantenha o `.env`, a pasta `secrets/` e os backups fora de repositórios.

Detalhes nas decisões [0003 (segredos)](docs/decisoes/0003-segredos-cifrados.md), [0007 (segundo fator)](docs/decisoes/0007-segundo-fator-no-chat.md) e [0008 (setup)](docs/decisoes/0008-setup-pelo-navegador.md).

## Recuperar o acesso

| Situação | O que fazer |
| --- | --- |
| **Esqueci a senha** ou **perdi o acesso ao chat** | No `.env`, coloque `SETUP_REOPEN=true` e rode `docker compose up -d`. Abra http://localhost:8080/setup, cole o `SETUP_TOKEN` e defina a senha nova e/ou outro bot. Os dados continuam todos lá. Ao terminar, **remova** `SETUP_REOPEN` (ou deixe `false`) |
| Apaguei o `SETUP_TOKEN` | Rode `make init`: ele gera um novo se a linha estiver vazia |
| O bot está fora do ar e o login diz "não foi possível enviar o código" | O app tenta religar o bot sozinho (a cada poucos segundos, até 5 minutos entre tentativas). Emergência: `DASHBOARD_2FA=off` no `.env` e `docker compose up -d` faz o login pedir só a senha |

> Para reabrir o setup **uma segunda vez**, reinicie uma vez com `SETUP_REOPEN=false` e depois com `true`: um `SETUP_REOPEN` esquecido ligado não reabre o setup a cada reinício.

## Backup e restauração

O serviço `backup` grava uma cópia completa do banco em `./backups` ao subir e a cada 24 horas, guardando as 14 mais recentes (`BACKUP_INTERVAL_HOURS` e `BACKUP_KEEP` no `.env`).

| Comando | O que faz |
| --- | --- |
| `make backup` | Backup na hora |
| `make restore FILE=backups/finassist-AAAAMMDD-HHMMSS.dump` | Restaura (substitui os dados atuais) |

- Os arquivos ficam fora do git, com permissão 600, e contêm todos os seus dados financeiros: **criptografe antes de enviar a qualquer serviço** (`age` ou `gpg --symmetric arquivo.dump`) e guarde uma cópia **fora da máquina**.
- O serviço roda com o usuário `1000:1000`. Se o seu for outro (`id -u`, `id -g`), defina `BACKUP_UID` e `BACKUP_GID` no `.env`.
- Para testar uma restauração sem risco, restaure num banco à parte: `docker compose exec -T postgres psql -U finassist -d finassist -c "CREATE DATABASE teste"` e `docker compose exec -T postgres pg_restore -U finassist -d teste --no-owner < backups/<arquivo>.dump`.

## Atualizando uma instalação existente

```bash
git pull
docker compose up -d --build
```

**Migrations novas:** o Postgres só roda os scripts de `backend/migrations/` quando o banco é criado. Em um banco existente, aplique à mão os que faltam (rodar de novo não estraga nada):

```bash
for f in backend/migrations/01[0-9]_*.sql; do docker compose exec -T postgres psql -q -U finassist -d finassist < "$f"; done
```

| Migration | Para quê |
| --- | --- |
| `010` a `014` | Revisão, metas, histórico do Coach, decisões "cancelei", regras de categoria |
| `015` | Bancos, logos e dados dos cartões (depois, rode uma sincronização) |
| `016`, `017` | Página Configurações e o histórico de alterações |
| `018` | Idade real dos dados do banco |
| `019` | Setup no navegador e recuperação com `SETUP_REOPEN` |

**Quem já usava tudo pelo `.env`** (senha em `DASHBOARD_PASSWORD`, Telegram em `TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID`, Pluggy, Gemini) não precisa mudar nada: o app não mostra o setup e o login continua igual. O que for salvo pela página Configurações passa a valer mais que o `.env`. Para poder usar a recuperação com `SETUP_REOPEN`, rode `make init` uma vez (ele só acrescenta o que falta).

<details>
<summary>Configurar o Telegram pelo <code>.env</code>, sem o setup</summary>

1. Crie o bot no [@BotFather](https://t.me/BotFather) como no [passo 1.3](#13-criar-o-seu-bot-no-telegram).
2. Descubra o seu ID numérico: abra o [@userinfobot](https://t.me/userinfobot), toque em **Iniciar** e copie o número da linha `Id`.
3. No `.env`:

```env
CHANNEL=telegram
TELEGRAM_BOT_TOKEN=123456789:AAH4r9xYz...
TELEGRAM_CHAT_ID=987654321
DASHBOARD_PASSWORD=uma-senha-com-12-ou-mais
```

4. `docker compose up -d`. O log (`docker compose logs -f app`) mostra `canal Telegram ativo`.
</details>

## Comandos úteis e documentação

```bash
make init                      # prepara uma instalação nova (.env, chave mestra e SETUP_TOKEN)
docker compose up -d --build   # sobe (ou atualiza) tudo
docker compose logs -f app     # acompanha o log do app
docker compose down            # para os containers (os dados ficam)
make backup                    # backup na hora
make db-shell                  # abre o psql no banco
```

| Documento | Conteúdo |
| --- | --- |
| [docs/dashboard.md](docs/dashboard.md) | Cada tela do dashboard em detalhes |
| [docs/uso-do-chat.md](docs/uso-do-chat.md) | Exemplos de mensagens para o bot |
| [docs/coach.md](docs/coach.md) | O que o Coach envia à IA e os limites |
| [docs/regras-de-calculo.md](docs/regras-de-calculo.md) | Como os números são calculados |
| [docs/api.md](docs/api.md) | API do dashboard (rotas, autenticação, setup) |
| [docs/arquitetura.md](docs/arquitetura.md) | Camadas, pacotes e fluxos do código |
| [docs/decisoes/](docs/decisoes/README.md) | Decisões de arquitetura (ADRs) |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Rodar os testes e contribuir |

**Tecnologias:** Go, PostgreSQL, React + TypeScript (Vite), Telegram Bot API, Meu Pluggy (Open Finance), Google Gemini (opcional), Docker Compose.

## Licença

MIT
