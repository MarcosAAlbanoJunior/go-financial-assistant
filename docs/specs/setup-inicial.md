# Spec: Setup inicial pelo navegador

- **Status:** implementada (decisões na seção 12). Veja "Resultado" ao fim.
- **Origem:** conversa de 05/10/2026. Hoje subir o app exige preencher um `.env` longo (senha, canal, Gemini, Pluggy...) sem guia, e o app nem sobe se faltar algo obrigatório.
- **Depende de:** [autenticacao-otp.md](autenticacao-otp.md) (código no chat), já implementada.

## 1. Objetivo

Trocar o `.env` longo por um **assistente no navegador**, prático, rápido e didático, que leva a pessoa do zero até o dashboard funcionando, mostrando passo a passo o que fazer fora do app (criar o bot no Telegram, mandar o primeiro /start).

- O `.env` fica só com o que **não pode** sair dele (seção 3).
- O setup é protegido por um **token definido no `.env`**: só quem tem acesso ao servidor consegue fazê-lo.
- O setup só termina com um **canal configurado e confirmado**. Na primeira versão, **só Telegram**; o WhatsApp continua pelo `.env` como hoje. O resto (Gemini, Open Finance, nomes, resumo) fica para a página de Configurações, que já existe.
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
| `COMPOSE_PROFILES` e as variáveis da Evolution | só para quem usa WhatsApp, que nesta versão continua pelo `.env` |

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

Nesta versão, só Telegram. A tela diz em uma linha que quem prefere WhatsApp configura pelo `.env` (com link para o README), e segue.

#### Telegram

1. **Criar o bot.** Instruções numeradas: abrir o @BotFather, mandar `/newbot`, escolher nome e usuário, copiar o token. Campo para colar o token; ao colar, o app testa (`getMe`) e mostra "Bot @nome_do_bot encontrado ✓" ou o erro em português ("token inválido: confira se copiou inteiro").
2. **Descobrir o seu ID sem ferramenta externa.** "Abra o seu bot no Telegram (botão com o link `t.me/nome_do_bot`) e mande `/start`." A tela espera (consultando `getUpdates` a cada 2 s, por até 5 min) e mostra "Recebemos a mensagem de **Fulano** (@usuario). É você?" com Sim/Não. Isso dispensa o @userinfobot.
   - Só a **primeira conversa privada** depois de abrir o passo conta. Mensagens de grupo são ignoradas.
   - Se outra pessoa mandar `/start` antes, o "Não, não sou eu" descarta e continua esperando.
3. **Confirmar.** O app manda ao chat um código de 6 dígitos (mesmas regras do OTP) e a pessoa digita na tela. Confirmado, o canal está provado: funciona **e** é dela.

### Passo 3: Pronto

- Grava o canal (incluindo `CHANNEL`, que passa a ser uma configuração), marca o setup como concluído e **abre a sessão** do dashboard direto (a pessoa acabou de provar senha, token e chat).
- **Liga o canal na hora, sem reiniciar** (seção 7.1).
- Mostra **próximos passos opcionais**, cada um com link para o grupo certo de Configurações e uma frase do que ganha:
  - **Gemini** (opcional): entender mensagens livres no chat, ler recibos, Coach e sugestões de categoria. Sem ele, o chat funciona com os comandos e caminhos sem IA;
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
| Fechou na tela "Pronto" | Já está configurado: abre o login normal (senha + código) |

Regras que valem em todos os casos:

- **Sem canal confirmado, não está pronto.** Senha salva sozinha não libera o dashboard nem o login.
- **Voltar sempre exige o token**, mesmo com senha salva: a senha só passa a valer para entrar quando o setup termina. Assim, quem descobrir uma senha salva no meio não entra por ela.
- **Nenhum dado de passo incompleto vira configuração ativa.** O token do bot e o ID ficam como rascunho do setup até a confirmação do código; só então são gravados como `TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID`/`CHANNEL`.
- Trocar o bot no meio (colar outro token) descarta o ID recebido e recomeça a espera do `/start`.

## 7. Segurança

- Token: só no `.env`, colado em campo (nunca na URL), comparação em tempo constante, 5 tentativas por minuto por IP, mínimo de 24 caracteres.
- Sessão de setup separada da sessão do dashboard (cookie `fa_setup`), que não dá acesso a nenhuma outra rota.
- Rotas `/api/setup/*` exigem JSON e mesma origem (como o login). Depois de concluído, respondem `404`.
- O código de confirmação do canal segue as regras do OTP (6 dígitos, 5 min, 3 tentativas, teto de envios).
- No Telegram, só o chat que mandou `/start` **e** confirmou o código vira dono; o bot ignora qualquer outro, como hoje.
- Cada passo concluído vai para o histórico de alterações (sem valores).

### 7.1 Ligar o canal sem reiniciar

Tudo o que usa o chat (resumo semanal, relatório mensal, avisos de segurança, código do segundo fator) já recebe um `ports.Messenger`. Em vez de passar o cliente do Telegram, o app passa um **canal trocável**: um `ports.Messenger` que guarda o cliente num `atomic.Pointer` e responde `ErrNoChannel` enquanto não há canal. Ele é criado no boot (vazio, se não há canal) e, ao concluir o setup, recebe o cliente já validado e sobe o bot. Ninguém mais precisa saber se o canal subiu agora ou no boot.

Cuidados (cada um vira teste):

1. **Nada meio pronto à vista.** O cliente, o dono e o bot são montados por inteiro e só então publicados no ponteiro (troca atômica). Sem leitura de `func` ou campo sendo escrito por outra goroutine (`go test -race` no pacote).
2. **Liga uma vez só.** Concluir é idempotente: um `sync.Mutex` + a marca "concluído" gravada **na mesma transação** que o canal. Duas requisições de "concluir" ao mesmo tempo: uma liga, a outra recebe "já concluído". Nunca dois bots fazendo polling com o mesmo token (o Telegram devolve 409 e um dos dois perde mensagens).
3. **Ordem segura.** Confirma o código → grava canal + "concluído" (transação) → para a espera do `/start` do setup → publica o canal → sobe o bot → abre a sessão. Se gravar falhar, nada liga. Se o bot falhar ao subir depois de gravado, o setup está concluído e o app tenta de novo em segundo plano (item 5); não volta a abrir o setup.
4. **O bot começa depois do `/start` do setup.** A espera do setup e o bot usam `getUpdates`; o bot recebe o offset seguinte ao último update visto pelo setup, para não reprocessar o `/start` nem mensagens de terceiros que chegaram durante o setup. Só o ID confirmado é dono (como hoje).
5. **Canal que não sobe não rebaixa a segurança.** Hoje, se o Telegram falha ao subir (token errado ou a API fora no momento do boot), o `DASHBOARD_2FA=auto` passa a aceitar **só a senha** até o próximo reinício. Com o canal trocável, isso muda: o segundo fator fica ativo sempre que **há canal configurado**, mesmo que ele não tenha subido ainda. Login sem canal no ar responde `503` (falha fechada) e o app tenta ligar o canal de novo em segundo plano (com espera crescente, até 5 min). Quem ficou trancado por um token errado usa `DASHBOARD_2FA=off` ou `SETUP_REOPEN` (seção 8). Isso fecha uma brecha da implementação atual do OTP e precisa ser registrado na ADR 0007.
6. **Os valores vêm do banco, não da requisição.** Ao concluir, o canal é montado a partir do rascunho já gravado e confirmado (token cifrado, ID do `/start` confirmado), nunca de campos enviados na requisição de "concluir".

## 8. Reabrir o setup (recuperação)

Para quem perdeu a senha ou o chat: `SETUP_REOPEN=true` no `.env` + reiniciar reabre o setup (com o mesmo `SETUP_TOKEN`) para **redefinir a senha e/ou o canal**, mantendo todos os dados. Ao concluir, o app avisa no log para remover o `SETUP_REOPEN`; enquanto ele estiver ligado e o setup concluído, nada acontece (não reabre em loop). Isso substitui, para a senha, o "editar o `.env`"; o `DASHBOARD_2FA=off` continua existindo para emergências.

## 9. Mudanças por camada

**Backend**
- `config`: `GEMINI_API_KEY` e as variáveis do canal deixam de ser obrigatórias; `CHANNEL` vira configuração registrada (só gravada pelo setup); novos `SETUP_TOKEN` e `SETUP_REOPEN`.
- **Gemini opcional**: o app sobe sem ele. O que depende dele (interpretar mensagens livres e recibos, Coach, sugestões de categoria) responde "configure o Gemini nas Configurações" em vez de falhar, e os caminhos sem IA (comandos, resumo, saldos, sincronização, dashboard) seguem normais. **Levantar todos os pontos** que hoje assumem o cliente do Gemini. Salvar a chave nas Configurações passa a ligá-lo sem reiniciar, se for simples (o modelo do Coach já muda a quente); senão, pede reinício como hoje.
- `internal/setup`: estado do setup (passos, rascunhos), regras de "configurado", sem HTTP.
- Migration nova: `dashboard_owner` (hash da senha, quando o setup terminou) e o rascunho do setup.
- Senha no banco com argon2id; `checkPassword` passa a aceitar o hash do banco ou a variável.
- Telegram: método para esperar o `/start` (getUpdates com offset), usado só no setup, antes de o bot subir.
- Canal trocável (seção 7.1) no lugar do `messenger` passado em `startJobs`; `SecondFactor.Active` passa a olhar "há canal configurado", e `Send` usa o canal trocável.

**Frontend**
- `features/setup`: as telas, a barra de progresso, a espera do `/start` e a tela final com os próximos passos.
- Redirecionamento para `/setup` quando a API responde `{"setup": "required"}`.

**Outros**
- `make init`; `.env.example` enxuto (só o que fica no ambiente, com o resto comentado como opcional).
- README com o caminho novo em 3 linhas (`make init`, `docker compose up -d`, abrir o navegador), mantendo o do `.env` completo como alternativa.
- ADR: "Setup pelo navegador, liberado por token no ambiente".

**Migrations:** hoje elas só rodam quando o banco é criado (`docker-entrypoint-initdb.d`); bancos existentes aplicam à mão. Esta spec não muda isso, mas o setup é o primeiro recurso que **novas instalações** dependem de uma migration nova, então funciona. Rodar as migrations ao subir o app seria uma melhoria separada.

## 10. Plano

1. App sobe sem Gemini e sem canal (config opcional + o que depende avisa). Testes.
2. Canal trocável + segundo fator com falha fechada e nova tentativa em segundo plano (seção 7.1, itens 1 e 5). Testes com `-race`.
3. `internal/setup` + migration + senha com argon2id. Testes.
4. API do setup: token, passos, rascunho, Telegram (`getMe`, espera do `/start`, código), concluir ligando o canal, regras de "configurado". Testes de handler para cada linha da tabela da seção 6 e para os itens 2, 3, 4 e 6 da seção 7.1.
5. Front do setup.
6. Próximos passos e `SETUP_REOPEN`.
7. `make init`, `.env.example`, README, ADRs (nova e 0007), CHANGELOG.

Cada passo com `go test ./...`, `go vet ./...`, golangci-lint, `npx tsc -b`, `npx oxlint` e `npx vitest run` verdes.

## 11. Testes de aceitação

- Instalação nova com `make init`: do `docker compose up -d` ao dashboard só pelo navegador, sem editar o `.env` à mão e sem reiniciar; o bot responde no Telegram logo depois de concluir.
- Instalação sem `GEMINI_API_KEY`: sobe, o dashboard funciona, e no chat o que precisa de IA explica como ligar.
- Duas requisições de concluir ao mesmo tempo: um bot só.
- Telegram fora do ar no boot com canal configurado: login responde 503 (não aceita só a senha) e o canal liga sozinho quando o Telegram volta.
- Instalação atual com `.env` completo: nenhuma tela de setup; login como antes.
- Sem `SETUP_TOKEN`: a tela só explica como definir.
- Token errado 6 vezes num minuto: 429.
- Cada linha da tabela da seção 6.
- Outra pessoa manda `/start` ao bot durante o setup: aparece o nome dela, "não sou eu" descarta.
- Concluído: `/api/setup/*` responde 404; o login pede senha e código.
- `SETUP_REOPEN=true`: permite trocar a senha e o canal, mantém os dados.

## 12. Decisões (revisão de 05/10/2026)

1. **Concluir liga o canal na hora, sem reiniciar** (seção 7.1). O custo é um passo a mais no plano (o canal trocável), que também fecha a brecha do "canal não subiu, entra só com senha".
2. **Gemini é opcional**, fora do setup: aparece só nos próximos passos. Há caminhos sem IA.
3. **Só Telegram** na primeira versão. WhatsApp continua pelo `.env`; entra no setup numa versão futura (Evolution, QR code na tela, código pelo WhatsApp).

## 13. Resultado

Implementada como descrita. Diferenças e descobertas:

- **A espera do `/start` é puxada pela tela**, não uma goroutine do servidor: cada `POST /api/setup/telegram/poll` faz um `getUpdates` sem espera a partir do offset salvo no rascunho. Não sobra nada rodando se a pessoa fecha a aba, e "parar a espera" ao concluir é um `RWMutex` (consultas em andamento terminam antes de o bot começar a ler o Telegram). Ao colar o token, o offset começa depois da última mensagem que o bot já tinha, então um `/start` antigo não conta.
- **"Sim, sou eu" é um passo próprio** (`accept`), que manda o código; "mandar outro código" é repetir o `accept` (mesmo intervalo de 1 minuto do OTP). "Não sou eu" também cancela um código já enviado.
- **A senha nova fica no rascunho** (`setup_draft.password_hash`) e só vai para `dashboard_owner` na conclusão: uma senha salva no meio nunca vale para entrar, nem num setup reaberto que foi abandonado.
- **Conclusão em dois cadeados:** o `sync.Mutex` do processo e, no banco, a checagem otimista do dono (`UPDATE ... WHERE setup_completed_at IS NOT DISTINCT FROM $esperado`), que também cobre dois processos. Testado com 4 confirmações simultâneas (uma liga, as outras recebem erro) e no banco real.
- **Canal no `.env` sem senha** (quem tinha a API desligada): o setup só pede a senha e conclui mantendo o canal (`POST /api/setup/keep`); não deixa ligar outro bot por cima (seriam dois). Na reabertura dá para manter o canal de antes ou ligar outro bot, e o canal de antes não sobe no boot enquanto o setup estiver reaberto.
- **`CHANNEL` é uma configuração interna** do catálogo (`Internal`): o setup grava, a página de Configurações não mostra, não altera nem restaura. O canal da página passa a vir dela, então o Telegram aparece logo depois do setup, sem pedir reinício (`settings.Service.Adopt`).
- **A chave do Gemini vale sem reiniciar** (o cliente troca a chave com o app rodando). Sem ela, o chat responde como ligar e o Coach/Classificar mostram "configure a chave" no lugar do aviso do plano pago.
- **Telegram que volta sozinho:** primeira tentativa no boot e depois em 5 s, 10 s, 20 s... até 5 min entre tentativas.
- **`make init`** está em `scripts/init.sh`. Se o `.env` já usa `APP_SECRET_KEY` (variável), ele não cria o arquivo de chave: trocar a chave tornaria ilegíveis os segredos salvos.
- **Proxy do Vite:** a forma curta `'/api': 'http://...'` liga `changeOrigin` e o modo de desenvolvimento recusava as escritas (403 na checagem de mesma origem). Corrigido junto.
- Código: `internal/setup` (estado e regras), `internal/infra/db/setup_repository.go`, `internal/infra/http/setup_api.go` e `setup_session.go`, `internal/app/switch.go`, `setup.go`, `channel.go` e `retry.go`, `internal/auth/password.go` (argon2id), `frontend/src/features/setup`, migration `019_create_dashboard_owner.sql` e ADR 0008.
