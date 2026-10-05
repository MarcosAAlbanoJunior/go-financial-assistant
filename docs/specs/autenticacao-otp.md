# Spec: Segundo fator por código no chat (OTP)

- **Status:** implementada. Veja "Resultado" ao fim.
- **Origem:** conversa de 05/10/2026. A senha sozinha é o único portão do dashboard; se vazar ou for adivinhada, abre tudo (saldos, transações, segredos cifrados).
- **Natureza:** funcionalidade nova de segurança. Muda o login e a confirmação das Configurações; o resto da API não muda.

## 1. Objetivo

Somar à senha um **segundo fator**: um código de uso único enviado ao chat que a pessoa já configurou (Telegram ou WhatsApp). Assim:

- uma senha vazada ou adivinhada não basta para entrar;
- a senha não precisa ser longa demais, porque o brute force esbarra no código;
- uma sessão roubada (cookie) não basta para mexer no que é sensível nas Configurações;
- não há cadastro novo: a identidade da pessoa é o ID/número do canal que ela já definiu no `.env` ou nas Configurações.

**Não objetivos:** login por Google ou outro provedor, TOTP (app autenticador), passkeys, vários usuários, sessões que sobrevivem a reinício, revogação de sessões. Passkey fica como evolução possível: o cookie de sessão atual serve para os dois.

## 2. Premissas

- O app é de **uma pessoa só**, em VPS ou máquina pessoal.
- Quem usa o dashboard **sempre tem um canal** (Telegram ou WhatsApp).
- A segurança do segundo fator é a da conta do chat. A documentação recomenda ligar a verificação em duas etapas do Telegram/WhatsApp.

## 3. Login em dois passos

```
POST /api/login        { password }            → senha certa e 2FA ativo: envia o código, responde { step: "code" }
POST /api/login/code   { code }                → código certo: emite o cookie de sessão (igual ao de hoje)
```

1. `POST /api/login` confere a senha **como hoje** (mesmo limite de 5 por minuto por IP). Se estiver certa e o 2FA estiver ativo, **não** emite a sessão: cria um desafio e envia o código ao chat. A resposta traz `{ "step": "code" }` e um cookie `fa_challenge` (`HttpOnly`, `SameSite=Strict`, `Secure` atrás de HTTPS, 5 min) com o id do desafio. Com o 2FA inativo, o fluxo é o de hoje.
2. `POST /api/login/code` recebe o código, confere contra o desafio do cookie e, se bater, emite `fa_session`.

**Por que o código só sai depois da senha certa:** quem não sabe a senha não consegue fazer o app mandar mensagens ao seu chat (sem spam, sem como usar o bot para incomodar).

### Regras do código

| Regra | Valor |
| --- | --- |
| Formato | 8 dígitos, de `crypto/rand` |
| Validade | 5 minutos |
| Uso | único; depois de usado ou expirado o desafio some |
| Tentativas erradas | 3 por desafio; na terceira o desafio morre e é preciso errar/acertar a senha de novo |
| Reenvio | permitido 60 s depois do envio; o código novo invalida o anterior |
| Teto de envios | 10 por hora no total, somando login e confirmações (não por IP: o app é de uma pessoa só) |
| Guarda | só o hash (SHA-256) do código, em memória; comparação em tempo constante |
| Estado | em memória do processo. Reiniciar o app derruba os desafios, como já derruba as sessões |

Com 8 dígitos (100 milhões de combinações), 3 tentativas por código e 10 códigos por hora, adivinhar o código é inviável.

### Falha ao enviar

Se o canal recusar ou estiver fora, `POST /api/login` responde `503 "não foi possível enviar o código ao chat"` e **não** emite sessão (falha fechada). Para isso o envio do código precisa **devolver erro**: hoje `Notify` (`settings_api.go`) é "dispare e esqueça". Criar um `SendCode(ctx, text) error` sobre o mesmo canal, sem mexer no `Notify`.

### Avisos no chat

- Login concluído: `🔓 Login no dashboard (IP, 05/10 14:32).`
- Código errado esgotado: `⚠️ 3 códigos errados no login do dashboard.` (no máximo um aviso a cada 10 min, como o aviso de senha errada de hoje).

## 4. Quando o 2FA está ativo

Variável `DASHBOARD_2FA`, só no ambiente (como a senha):

| Valor | Efeito |
| --- | --- |
| `auto` (padrão) | ativo quando há canal pronto para receber (Telegram com ID autorizado, ou WhatsApp com número). Sem canal, só senha |
| `off` | desligado; só senha |

- **Primeiro acesso.** O canal se configura dentro do dashboard. Antes de existir canal, o `auto` deixa entrar só com senha, e a página de Configurações mostra um aviso fixo: "Segundo fator desligado: configure o canal". Ao configurar o canal, o 2FA passa a valer no próximo login.
- **Aviso permanente.** Com o 2FA inativo, o painel mostra o aviso acima e o app registra um `WARN` no log ao subir.
- **Recuperação.** Se o canal ficar fora do ar ou a pessoa perder o acesso a ele, quem tem acesso ao servidor põe `DASHBOARD_2FA=off` no `.env` e reinicia. Não se cria comando nem link de emergência: quem tem o servidor já controla a máquina.

## 5. Confirmação das Configurações

Hoje, mudar ou restaurar o que é sensível pede a **senha de novo** (`confirmPassword`). Passa a pedir um **código novo no chat**, quando o 2FA está ativo (sem 2FA, continua a senha, como hoje).

```
POST /api/settings/confirm   {}                  → envia o código; responde { step: "code" } + cookie fa_confirm
PUT  /api/settings           { ..., code }       → confere o código (uso único) e salva
POST /api/settings/reset/{key} { code }          → idem
```

- Mesmas regras de código da seção 3 (8 dígitos, 5 min, 3 tentativas, uso único), com **desafio próprio** (cookie `fa_confirm`), preso à sessão atual.
- Quando a mudança é o **próprio canal** (ID do Telegram, número do WhatsApp, token do bot), o código vai para o **canal que está salvo antes da mudança**, nunca para o novo. Senão, quem está com a sessão roubada trocaria o canal para o seu e receberia o código.
- O código **substitui** a senha nessa confirmação: com a sessão já aberta, a senha de novo adiciona pouco, e o código no chat é o que uma sessão roubada não tem.
- O aviso de "tentativa errada" no chat e o histórico de alterações continuam como estão.

## 6. Mudanças por camada

**Backend** (`internal/infra/http` e novo `internal/auth`):
- `internal/auth`: `Challenges` (criar, verificar, reenviar, expirar) puro e testável, com relógio e gerador de códigos injetáveis. Sem dependência de HTTP.
- `session.go` continua só com o cookie de sessão; `auth_api.go` ganha `loginCode`; `settings_security.go` troca `confirmPassword` por `confirm` (senha ou código, conforme o 2FA).
- `config`: lê `DASHBOARD_2FA` (valida `auto`/`off`).
- `app/jobs.go`: expõe o `SendCode` e a pergunta "há canal pronto?".

**Frontend** (`features/auth`, `features/settings`):
- Login em duas telas: senha → código. Campo com `inputMode="numeric"` e `autoComplete="one-time-code"`, contagem para reenviar, mensagem clara para código expirado ou errado.
- Configurações: o modal de confirmação passa a pedir o código (e a ter "Reenviar").
- Aviso fixo quando o 2FA está inativo.

**Documentação:** `docs/api.md` (rotas e cookies), `docs/dashboard.md`, `README.md` (`DASHBOARD_2FA`, recomendação de ligar a verificação em duas etapas do chat), nova ADR `0007-segundo-fator-no-chat.md`, `CHANGELOG.md`.

## 7. Plano

1. **`internal/auth`** com testes (validade, tentativas, uso único, reenvio, teto de envios).
2. **Login em dois passos** no backend + `SendCode` com erro. Testes de handler com um canal falso (sucesso, falha de envio, 3 erros, expirado).
3. **Frontend do login** em dois passos.
4. **Confirmação das Configurações** por código (backend e front), incluindo "código vai ao canal antigo".
5. **Avisos** (login concluído, códigos errados, 2FA inativo) e `DASHBOARD_2FA`.
6. **Documentação** e ADR.

Cada passo termina com `go test ./...`, `go vet ./...`, `npx tsc -b`, `npx oxlint` e `npx vitest run` verdes, em commits pequenos.

## 8. Testes de aceitação

- Senha errada: 401, nenhuma mensagem no chat.
- Senha certa com 2FA: mensagem no chat, sem cookie de sessão.
- Código certo dentro de 5 min: sessão. O mesmo código de novo: recusado.
- 3 códigos errados: desafio morto, novo login exige a senha de novo.
- Código expirado: recusado.
- Canal fora do ar: 503 e nenhuma sessão.
- 11º envio na mesma hora: 429.
- `DASHBOARD_2FA=off` ou sem canal: login só com senha, como hoje.
- Configurações: salvar segredo sem código: recusado; com código: salvo; trocar o ID do Telegram manda o código ao ID antigo.

## 9. Pontos a confirmar

1. **Confirmação sem senha.** A seção 5 troca a senha pelo código. Se preferir os dois (senha **e** código), é só somar; o custo é um passo a mais a cada mudança sensível.
2. **Tamanho do código.** 8 dígitos por padrão; 6 é mais cômodo de digitar e ainda seguro com 3 tentativas.
3. **Duração da sessão.** Os 7 dias atuais continuam. Com o 2FA, dá para ser mais generoso (30 dias) ou mais curto (1 dia), conforme o quanto a pessoa quer digitar o código.

## 10. Resultado

Implementada como descrita, com os padrões da seção 9 (só código na confirmação, 8 dígitos, sessão de 7 dias). Diferenças e descobertas:

- **Teto de 10 envios por hora** em vez de 5: login e confirmações dividem o teto, e configurar vários segredos seguidos esgotaria 5 rápido.
- **Sem cookie `fa_confirm`:** o desafio de confirmação é preso ao próprio cookie de sessão, então não precisa de outro. Há no máximo um desafio vivo por tipo (login, confirmação); pedir de novo substitui o anterior.
- **Reenviar no login** é repetir `POST /api/login` com a senha (o front guarda a senha em memória durante os dois passos).
- **"Código vai ao canal antigo"** sai de graça: o canal e o dono são lidos ao subir e mudar qualquer um deles pede reinício. O envio usa sempre o canal em execução.
- **Canal pronto** = o canal subiu (`messenger != nil`). O token errado do Telegram ou o WhatsApp sem conexão na subida deixam o `auto` só com senha, com `WARN` no log e aviso nas Configurações.
- **Aviso de login concluído** vai em todo login, com ou sem segundo fator.
- `GET /api/settings` também devolve `secondFactor`, porque a página de Configurações não pode importar `features/auth` (regra de arquitetura do front).
- Código: `internal/auth` (desafios), `internal/infra/http/second_factor.go` e `auth_api.go`, `internal/app/jobs.go` (envio), `frontend/src/features/auth` e `features/settings/components/ConfirmDialog.tsx`.
