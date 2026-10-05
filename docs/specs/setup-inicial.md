# Spec: Setup inicial pelo navegador

- **Status:** proposta, aguardando revisão.
- **Origem:** conversa de 05/10/2026. Hoje subir o app exige preencher um `.env` longo (senha, canal, Gemini, Pluggy...) sem guia, e o app nem sobe se faltar algo obrigatório.
- **Depende de:** [autenticacao-otp.md](autenticacao-otp.md) (código no chat), já implementada.

## 1. Objetivo

Trocar o `.env` longo por um **assistente no navegador**, prático, rápido e didático, que leva a pessoa do zero até o dashboard funcionando, mostrando passo a passo o que fazer fora do app (criar o bot no Telegram, conectar o WhatsApp).

- O `.env` fica só com o que **não pode** sair dele (seção 3).
- O setup é protegido por um **token definido no `.env`**: só quem tem acesso ao servidor consegue fazê-lo.
- O setup só termina com um **canal (Telegram ou WhatsApp) configurado e confirmado**. O resto (Gemini, Open Finance, nomes, resumo) fica para a página de Configurações, que já existe.
- Sair no meio não perde nada nem deixa brecha (seção 6).
- Quem já usa o app com `.env` completo **não vê o setup** e nada quebra.

**Não objetivos:** vários usuários, instalar o Docker por você, configurar HTTPS/proxy, migrar automaticamente o `.env` para o banco.

## 2. Por que um token no `.env` (e não setup aberto)

Um setup aberto deixa quem chegar primeiro virar dono da instância (o problema clássico de instalação do WordPress; há robôs que procuram instalações recém-subidas). O compose publica só em `127.0.0.1`, mas basta pôr um proxy ou túnel na frente antes de terminar para a janela abrir.

Com `SETUP_TOKEN` no `.env`, a prova de dono é a mesma de hoje (acesso ao servidor), sem o token passar por log, URL ou histórico do navegador: a pessoa **cola** o token num campo.

## 3. O que fica no ambiente

| Variável | Por quê |
| --- | --- |
| `DATABASE_URL` | o app precisa dela para chegar ao banco onde está o resto |
| `APP_SECRET_KEY_FILE` (ou `APP_SECRET_KEY`) | cifra os segredos no banco; não pode morar no banco que protege |
| `SETUP_TOKEN` | libera o setup (e a reabertura, seção 8) |
| `PORT`, `WEB_PORT` | infraestrutura |
| `DASHBOARD_2FA` | saída de emergência (`off`), precisa funcionar com o canal quebrado |
| `COMPOSE_PROFILES` | só para WhatsApp: decide se o Docker sobe a Evolution API (o app não liga containers) |

Todo o resto passa a ser **opcional no ambiente** e configurável no banco: senha (hash), canal, Telegram, WhatsApp, Gemini, Pluggy, nomes, resumo. A precedência continua a da ADR 0004 (salvo > ambiente > padrão), então um `.env` completo continua valendo.

### `make init`

Um comando para começar, que **nunca sobrescreve** o que existe:

1. cria `.env` a partir do `.env.example`, se não existir;
2. cria `secrets/app_secret_key` (como o `make secret-key`) e põe `APP_SECRET_KEY_FILE` no `.env`;
3. gera um `SETUP_TOKEN` aleatório (32 caracteres) e grava no `.env`;
4. imprime: "Rode `docker compose up -d`, abra http://localhost:8080 e cole o token do arquivo .env (linha SETUP_TOKEN)".

Quem não usar o `make` faz à mão (o README mostra o `openssl rand -hex 16`).

## 4. Quando o app está "configurado"

**Configurado = tem senha (banco ou `DASHBOARD_PASSWORD`) e tem canal confirmado (banco) ou canal completo no ambiente.**

- Instalações atuais, com `DASHBOARD_PASSWORD` e o canal no `.env`, já contam como configuradas: nunca veem o setup.
- Enquanto não configurado:
  - o dashboard só serve as rotas `/api/setup/*`; as demais respondem `409 {"setup": "required"}` e o front leva para `/setup`;
  - o bot e as rotinas que usam o chat não sobem (não há canal); a sincronização e o resto do app podem subir, mas sem efeito visível.
- **O app precisa subir sem `GEMINI_API_KEY` e sem as variáveis do canal.** Hoje `config.Load` as exige: passam a ser opcionais, e o que depende delas fica desligado com aviso (seção 9).

## 5. O passo a passo

Uma tela por passo, com barra de progresso ("Passo 2 de 4"), linguagem simples e, em cada passo, **o que fazer fora do app** em lista numerada. Cada passo salva ao avançar.

### Passo 0: Token

- Sem `SETUP_TOKEN` no ambiente: a tela só explica como definir (`make init` ou à mão) e reiniciar. Nada mais funciona.
- Sem chave mestra: avisa que falta `APP_SECRET_KEY_FILE` (`make secret-key`), porque o token do bot precisa ser guardado cifrado.
- Com tudo: campo "Cole o token de setup (está no arquivo .env, linha SETUP_TOKEN)". Certo, abre uma **sessão de setup** (cookie próprio `fa_setup`, `HttpOnly`, `SameSite=Strict`, 1 hora, renovada a cada passo).
- Limite de 5 tentativas por minuto por IP; comparação em tempo constante; token com no mínimo 24 caracteres (mais curto: a tela pede um maior).

### Passo 1: Senha do dashboard

- Senha + confirmação, mínimo de 12 caracteres, com medidor simples e a explicação: "Junto com ela, o login vai pedir um código no seu chat. Não precisa ser enorme, mas não use uma senha de outro site."
- Guardada como **hash argon2id** numa tabela própria (`dashboard_owner`, migration nova), nunca no `settings` nem em texto.
- Se `DASHBOARD_PASSWORD` estiver no ambiente, o passo mostra "Senha definida no .env" e pode ser pulado.

### Passo 2: Canal

Escolha com uma frase de cada: **Telegram (recomendado)**: só precisa criar um bot, leva 2 minutos. **WhatsApp**: usa a Evolution API num container a mais e um número conectado por QR code.

#### Telegram

1. **Criar o bot.** Instruções numeradas: abrir o @BotFather, mandar `/newbot`, escolher nome e usuário, copiar o token. Campo para colar o token; ao colar, o app testa (`getMe`) e mostra "Bot @nome_do_bot encontrado ✓" ou o erro em português ("token inválido: confira se copiou inteiro").
2. **Descobrir o seu ID sem ferramenta externa.** "Abra o seu bot no Telegram (botão com o link `t.me/nome_do_bot`) e mande `/start`." A tela espera (consultando `getUpdates` a cada 2 s, por até 5 min) e mostra "Recebemos a mensagem de **Fulano** (@usuario). É você?" com Sim/Não. Isso dispensa o @userinfobot.
   - Só a **primeira conversa privada** depois de abrir o passo conta. Mensagens de grupo são ignoradas.
   - Se outra pessoa mandar `/start` antes, o "Não, não sou eu" descarta e continua esperando.
3. **Confirmar.** O app manda ao chat um código de 6 dígitos (mesmas regras do OTP) e a pessoa digita na tela. Confirmado, o canal está provado: funciona **e** é dela.

#### WhatsApp

1. **Subir a Evolution API.** A tela testa se ela responde. Se não: instruções para pôr `COMPOSE_PROFILES=whatsapp` e `EVOLUTION_API_KEY` no `.env` e rodar `docker compose up -d`, com botão "Verificar de novo". (A chave da Evolution fica no ambiente porque o container dela também a lê.)
2. **Seu número.** Campo com máscara e exemplo (`55 11 99999-9999`).
3. **Conectar.** A tela mostra o **QR code** (o mesmo de `/admin/qrcode`, agora dentro do setup) com as instruções do WhatsApp (Aparelhos conectados → Conectar um aparelho), atualizando sozinho até a conexão abrir.
4. **Confirmar.** Código de 6 dígitos enviado por WhatsApp ao número, digitado na tela.

### Passo 3: Pronto

- Grava o canal (incluindo `CHANNEL`, que passa a ser uma configuração), marca o setup como concluído e **abre a sessão** do dashboard direto (a pessoa acabou de provar senha, token e chat).
- **Liga o canal sem reiniciar.** Se isso for complexo demais, a alternativa aceitável é reiniciar sozinho (o Docker sobe de novo) com uma tela "Ligando o seu assistente…" que espera e entra.
- Mostra **próximos passos opcionais**, cada um com link para o grupo certo de Configurações e uma frase do que ganha:
  - **Gemini** (entender mensagens livres no chat, ler recibos e o Coach): onde pegar a chave;
  - **Open Finance (Meu Pluggy)**: sincronizar bancos e cartões;
  - **Seus nomes** (`OWN_NAMES`): ignorar transferências entre contas suas;
  - **Resumo semanal**: dia e hora.
- Lembra: "Você já pode apagar o SETUP_TOKEN do .env (ou deixar: ele não abre mais nada)."

## 6. Sair no meio

O estado fica no banco a cada passo; nada importante fica só no navegador.

| Onde parou | O que acontece ao voltar |
| --- | --- |
| Antes de colar o token | Nada foi salvo. Começa do passo 0 |
| Token aceito, sem senha | Pede o token de novo (a sessão de setup expirou ou é outro navegador) e volta ao passo 1 |
| Senha salva, canal não escolhido | Token de novo; o passo 1 aparece como feito ("Senha definida ✓", com "trocar"); segue no passo 2 |
| Bot do Telegram validado, sem `/start` | O token do bot fica salvo (cifrado); volta no "mande /start" |
| `/start` recebido, código não digitado | O código expira em 5 min; a tela oferece "mandar outro código" (ou "não sou eu") |
| WhatsApp sem Evolution no ar | Volta no "subir a Evolution API" com as instruções |
| QR code não lido | Volta no QR code, que é gerado de novo |
| Fechou na tela "Pronto" | Já está configurado: abre o login normal (senha + código) |

Regras que valem em todos os casos:

- **Sem canal confirmado, não está pronto.** Senha salva sozinha não libera o dashboard nem o login.
- **Voltar sempre exige o token**, mesmo com senha salva: a senha só passa a valer para entrar quando o setup termina. Assim, quem descobrir uma senha salva no meio não entra por ela.
- **Nenhum dado de passo incompleto vira configuração ativa.** O token do bot e o ID ficam como rascunho do setup até a confirmação do código; só então são gravados como `TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID`/`CHANNEL`.
- Trocar de canal no meio (começou Telegram, quer WhatsApp) descarta o rascunho do outro.

## 7. Segurança

- Token: só no `.env`, colado em campo (nunca na URL), comparação em tempo constante, 5 tentativas por minuto por IP, mínimo de 24 caracteres.
- Sessão de setup separada da sessão do dashboard (cookie `fa_setup`), que não dá acesso a nenhuma outra rota.
- Rotas `/api/setup/*` exigem JSON e mesma origem (como o login). Depois de concluído, respondem `404`.
- O código de confirmação do canal segue as regras do OTP (6 dígitos, 5 min, 3 tentativas, teto de envios).
- No Telegram, só o chat que mandou `/start` **e** confirmou o código vira dono; o bot ignora qualquer outro, como hoje.
- Cada passo concluído vai para o histórico de alterações (sem valores).

## 8. Reabrir o setup (recuperação)

Para quem perdeu a senha ou o chat: `SETUP_REOPEN=true` no `.env` + reiniciar reabre o setup (com o mesmo `SETUP_TOKEN`) para **redefinir a senha e/ou o canal**, mantendo todos os dados. Ao concluir, o app avisa no log para remover o `SETUP_REOPEN`; enquanto ele estiver ligado e o setup concluído, nada acontece (não reabre em loop). Isso substitui, para a senha, o "editar o `.env`"; o `DASHBOARD_2FA=off` continua existindo para emergências.

## 9. Mudanças por camada

**Backend**
- `config`: `GEMINI_API_KEY` e as variáveis do canal deixam de ser obrigatórias; `CHANNEL` vira configuração registrada (só gravada pelo setup); novos `SETUP_TOKEN` e `SETUP_REOPEN`.
- App sobe sem Gemini: o que depende dele (interpretar mensagens livres e recibos, Coach, sugestões de categoria) responde "configure o Gemini" em vez de falhar. **Levantar todos os pontos** que hoje assumem o cliente do Gemini.
- `internal/setup`: estado do setup (passos, rascunhos), regras de "configurado", sem HTTP.
- Migration nova: `dashboard_owner` (hash da senha, quando o setup terminou) e o rascunho do setup.
- Senha no banco com argon2id; `checkPassword` passa a aceitar o hash do banco ou a variável.
- Telegram: método para esperar o `/start` (getUpdates com offset), usado só no setup, antes de o bot subir.
- WhatsApp: QR code servido pela API do setup (reaproveitando o `qrcode_handler`).
- Ligar o canal depois do setup sem reiniciar, ou reinício automático (seção 5, passo 3).

**Frontend**
- `features/setup`: as telas, a barra de progresso, a espera do `/start` e do QR code, e a tela final com os próximos passos.
- Redirecionamento para `/setup` quando a API responde `{"setup": "required"}`.

**Outros**
- `make init`; `.env.example` enxuto (só o que fica no ambiente, com o resto comentado como opcional).
- README com o caminho novo em 3 linhas (`make init`, `docker compose up -d`, abrir o navegador), mantendo o do `.env` completo como alternativa.
- ADR: "Setup pelo navegador, liberado por token no ambiente".

**Migrations:** hoje elas só rodam quando o banco é criado (`docker-entrypoint-initdb.d`); bancos existentes aplicam à mão. Esta spec não muda isso, mas o setup é o primeiro recurso que **novas instalações** dependem de uma migration nova, então funciona. Rodar as migrations ao subir o app seria uma melhoria separada.

## 10. Plano

1. App sobe sem Gemini e sem canal (config opcional + o que depende avisa). Testes.
2. `internal/setup` + migration + senha com argon2id. Testes.
3. API do setup: token, passos, rascunho, telegram (`getMe`, espera do `/start`, código), regras de "configurado". Testes de handler para cada linha da tabela da seção 6.
4. Front do setup (Telegram primeiro).
5. WhatsApp no setup (Evolution, QR code, código).
6. Concluir sem reiniciar (ou reinício automático), próximos passos, `SETUP_REOPEN`.
7. `make init`, `.env.example`, README, ADR, CHANGELOG.

Cada passo com `go test ./...`, `go vet ./...`, golangci-lint, `npx tsc -b`, `npx oxlint` e `npx vitest run` verdes.

## 11. Testes de aceitação

- Instalação nova com `make init`: do `docker compose up -d` ao dashboard só pelo navegador, sem editar o `.env` à mão (Telegram).
- Instalação atual com `.env` completo: nenhuma tela de setup; login como antes.
- Sem `SETUP_TOKEN`: a tela só explica como definir.
- Token errado 6 vezes num minuto: 429.
- Cada linha da tabela da seção 6.
- Outra pessoa manda `/start` ao bot durante o setup: aparece o nome dela, "não sou eu" descarta.
- Concluído: `/api/setup/*` responde 404; o login pede senha e código.
- `SETUP_REOPEN=true`: permite trocar a senha e o canal, mantém os dados.

## 12. Pontos a confirmar

1. **Concluir sem reiniciar ou com reinício automático.** Ligar o canal a quente é mais elegante, mas mexe na montagem do app (hoje o canal sobe uma vez em `startChannel`). O reinício automático é simples e já existe o botão de reiniciar nas Configurações.
2. **Gemini como passo opcional dentro do setup** (em vez de só nos próximos passos). Sem ele o chat não entende mensagens livres, que é o uso principal; talvez mereça um passo "pular por agora".
3. **WhatsApp no setup já na primeira versão** ou só Telegram primeiro (o WhatsApp exige mexer no `.env` de qualquer forma, por causa do container da Evolution).
