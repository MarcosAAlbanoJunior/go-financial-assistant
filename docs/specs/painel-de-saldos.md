# Spec: Painel de saldos (dashboard + Telegram)

- **Status:** proposta, aguardando aprovação
- **Mockup visual:** [painel-de-saldos-mockup.html](painel-de-saldos-mockup.html) (dados fictícios; abra no navegador)
- **Entrega em:** vários commits pequenos, na mesma feature (núcleo compartilhado, depois dashboard, depois Telegram)

## 1. Objetivo

Um painel único e bonito para responder, de relance, "quanto tenho agora?":

- o **saldo de cada banco** e o **total em conta**, com a proporção entre os bancos;
- **cada cartão de crédito**, separado (fatura, limite usado, disponível, vencimento), **sem somar cartões** entre si;
- ícone e cor de cada banco, estados claros (atualizado, desatualizado, negativo, vence logo);
- a mesma informação no **Telegram** com o comando `/saldos`, calculada pela mesma lógica.

**Não objetivos:** pagar fatura, mover dinheiro, histórico de saldo (a tela Investimentos já tem o do patrimônio), editar contas, mais de um usuário.

## 2. O que já existe e o que falta

| Necessidade | Hoje | Falta |
| --- | --- | --- |
| Saldo por conta, tipo BANK/CREDIT, final, limite, disponível, `updated_at` | tabela `accounts` (migration 006), tela "Contas e cartões" | — |
| **Qual banco** é cada conta | só `accounts.name`, inconsistente ("itau", "Banco Santander", "SANTANDER ELITE MASTER") e `item_id` | nome oficial, cor e logo do banco (conector do Pluggy) |
| Fatura (valor devido) do cartão | `accounts.balance` de CREDIT já guarda o "valor devido" (Pluggy: *"CREDIT: amount owed"*), mas a tela mostra só limite | usar e rotular como **fatura** |
| Fechamento, vencimento e pagamento mínimo | não sincronizados | `creditData.balanceCloseDate`, `balanceDueDate`, `minimumPayment` |
| Bandeira do cartão | não | `creditData.brand` (opcional) |
| Saldo investido automaticamente (Itaú) | não | `bankData.automaticallyInvestedBalance` (opcional, ver decisão 2) |
| Logos | não; o CSP do nginx só permite `img-src 'self' data:` | cache local do logo, servido pelo próprio app |
| Telegram | só texto, só `message` nos updates | comando `/saldos`; botões inline em etapa posterior |

Fontes consultadas: Pluggy `GET /items/{id}` (o objeto `connector` traz `name`, `imageUrl` e `primaryColor`) e `GET /accounts` (`balance`, `bankData`, `creditData`); Telegram Bot API (`callback_query` exige `allowed_updates` e `answerCallbackQuery`; envio de foto por multipart). Limites do Telegram (texto 4096, legenda 1024, `callback_data` 64 bytes, foto 10 MB) são de conhecimento prévio e **devem ser reconfirmados na implementação**.

## 3. Regras de cálculo (uma só fonte de verdade)

Tudo isso vive em um único serviço do usecase (`Balances`), usado pela API e pelo Telegram. Valores sempre em reais, calculados no backend.

- **Total em conta** = soma do `balance` das contas `BANK`. Conta com saldo negativo entra com o sinal.
- **Participação do banco** = saldo do banco ÷ soma dos saldos *positivos* (banco negativo mostra o valor, sem fatia na barra).
- **Cartão** (nunca somado entre cartões):
  - `fatura` = `balance` (valor devido);
  - `limiteUsado` = `limite − disponível` e `percentualUsado` = `limiteUsado ÷ limite`;
  - mesmos níveis de severidade da tela atual: ≥ 70% atenção, ≥ 90% crítico (ícone **e** texto, nunca só cor);
  - `diasParaVencer` = `vencimento − hoje`; fatura > 0 e vence em ≤ 3 dias → destaque de atenção; vencida → crítico;
  - limite ou disponível ausentes → "limite não informado pelo banco" (sem barra).
- **Atualização:** mostra `updated_at` por banco; mais de 36 h sem atualizar → rótulo "desatualizado" (o Meu Pluggy atualiza cerca de uma vez por dia).
- **Linha informativa "Faturas em aberto"** (decisão 1): soma das faturas, **à parte e sem subtrair do total em conta**.
- **Investimento automático do Itaú** (decisão 2): exibido sob o banco como "Aplicado automaticamente", **fora** do total em conta, para não duplicar o Patrimônio.

## 4. Dashboard

### 4.1 Layout (desktop)

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Painel                                    [👁 Ocultar valores] [↻ Sincronizar]│
├────────────────────────────────────────────────────────────────────────────┤
│  TOTAL EM CONTA                                                             │
│  R$ 12.345,67        2 bancos · atualizado hoje às 08:12                    │
│  ██████████████████████████▌ ▐███████████████████   (barra proporcional)    │
│  (S) Santander R$ 8.000,00 · 65%      (I) Itaú R$ 4.345,67 · 35%            │
│  Faturas em aberto, à parte: R$ 2.140,00                                    │
├──────────────────────────────────┬─────────────────────────────────────────┤
│ ▌(S) Santander        hoje 08:12 │ ▌(I) Itaú              hoje 08:12       │
│ Conta corrente · final 1234      │ Conta corrente · final 5678             │
│ R$ 8.000,00                      │ R$ 4.345,67                             │
│ ────────────────────────────     │ Aplicado automaticamente R$ 900,00      │
│ 💳 Elite Master · final 9012     │ ────────────────────────────            │
│ Fatura R$ 1.200,00 · vence 10/10 │ 💳 Click MC Platinum · final 3456       │
│ Limite usado ▓▓▓▓▓░░░░░ 42%      │ Fatura R$ 940,00 · ⚠ vence em 2 dias    │
│ Disponível R$ 5.800,00 de 10 mil │ Limite usado ▓▓▓▓▓▓▓▓░░ 78% ⚠ atenção   │
└──────────────────────────────────┴─────────────────────────────────────────┘
```

- **Hierarquia:** o banco é o cartão principal (logo + nome + atualização); dentro dele, a conta corrente e os cartões daquele banco. Assim "o que é do Itaú" fica junto.
- **Topo (hero):** total em conta em tipografia grande, barra proporcional por banco e legenda com logo, nome, valor e percentual. Sem rosca/pizza: com poucas partes, uma barra se lê melhor.
- **Móvel:** uma coluna; hero em cima, barra com legenda empilhada, bancos um sob o outro; alvos de toque ≥ 44 px.
- **Ocultar valores:** botão de olho que troca todos os valores por `R$ ••••` (e os percentuais continuam). Estado em `localStorage` (por navegador); começa visível. Útil para abrir o painel em público e para capturas de tela.
- **Sincronizar:** botão que chama a mesma sincronização do `/sync` (decisão 7).

### 4.2 Cores, ícones e identidade

- **Logo** do banco (cache local, ver 6.3) ou, sem logo, **monograma** (inicial sobre a cor do banco). Sempre com o **nome por extenso ao lado**: cor e logo nunca são o único sinal.
- **Cor do banco** (`primaryColor` do conector): faixa lateral do cartão, chip do logo e destaques. Texto sobre a cor é escolhido por contraste (preto ou branco, mínimo 4,5:1); no tema escuro, a cor é clareada até ter contraste com o fundo.
- **Atenção às cores vizinhas.** Medi as cores de marca dos seus dois bancos com o validador de paleta categórica: vermelho `#ec0000` e laranja `#ec7000` **reprovam** na distinção sem daltonismo (ΔE 13,4, mínimo 15) e o laranja fica abaixo de 3:1 de contraste no fundo claro. Regra decorrente: **as fatias da barra proporcional só usam a cor de marca quando cada par vizinho passa (ΔE ≥ 15 em OKLab)**; se não passar, a fatia do banco seguinte usa a cor da série categórica do app (`--series-N`, ordem fixa por banco), mantendo a cor de marca apenas na faixa, no chip e no logo. Fatias sempre com vão de 2 px entre si, nome e valor na legenda. Essa checagem é uma função pura e testada.
- **Cartões** em estilo "cartão de crédito": gradiente suave da cor do banco, bandeira em texto (ex.: "Mastercard") e o final do cartão.
- **Estados** (ícone + texto + cor): *atenção* (limite ≥ 70% ou vence em ≤ 3 dias), *crítico* (≥ 90% ou vencida), *desatualizado*, *saldo negativo*. Tokens de cor existentes (`--warning`, `--critical`), temas claro e escuro.
- **Vida:** ao carregar, números fazem contagem rápida (≈ 400 ms) e a barra cresce; tudo desligado com `prefers-reduced-motion`. Sem animação contínua.
- **Acessibilidade:** a barra e os medidores têm `role="meter"`/texto equivalente e há uma tabela (visão alternativa) com os mesmos valores; foco visível; ordem de leitura = ordem visual.

### 4.3 Estados da tela

Carregando (esqueleto dos cartões), erro com "tentar de novo", **sem Open Finance** ("configure o Open Finance", link para o README), banco sem cartão, cartão sem limite informado, banco desatualizado (rótulo e botão de sincronizar em destaque), tudo zerado.

### 4.4 Navegação

Nova página **Painel** (`/painel`), primeiro item do menu. "Contas e cartões" passa a redirecionar para ela (decisão 4) e a tile "Em conta" da Visão geral leva ao Painel.

## 5. Telegram

### 5.1 Comando

`/saldos` (também "saldos" e "painel"). Responde só ao dono, como todo o bot. Mensagem em HTML do Telegram (negrito e blocos), com emojis e barras de texto, para ficar legível também na notificação:

```
💰 <b>Seus saldos</b> · 02/10 09:12

<b>Em conta: R$ 12.345,67</b>
🏦 Santander  R$ 8.000,00  ██████░░░░ 65%
🏦 Itaú       R$ 4.345,67  ███░░░░░░░ 35%

💳 <b>Cartões</b>
• Santander Elite ·· 9012
  Fatura R$ 1.200,00 · vence 10/10 (em 8 dias)
  Limite usado ▓▓▓▓░░░░░░ 42% · disponível R$ 5.800,00
• Itaú Click ·· 3456
  Fatura R$ 940,00 · ⚠️ vence em 2 dias
  Limite usado ▓▓▓▓▓▓▓▓░░ 78% ⚠️ atenção

Faturas em aberto, à parte: R$ 2.140,00
🕘 Atualizado há 3 h · /sync para atualizar
```

Regras: todo texto vindo do banco passa por escape de HTML; mensagem sempre < 4096 caracteres (corta cartões extras com "e mais N"); bancos desatualizados ganham "⚠️ desatualizado há X h"; sem Open Finance, uma frase explicando.

### 5.2 Etapas

1. **T1 (esta feature):** comando `/saldos` com texto. Zero dependências novas.
2. **T2 (opcional):** botões inline "🔄 Sincronizar" e "👁 Ocultar" (exige tratar `callback_query`, `answerCallbackQuery` e `editMessageText`).
3. **T3 (só se o texto não bastar):** imagem do painel via `sendPhoto`. Exigiria desenhar em Go (`golang.org/x/image` e uma fonte OFL embutida). Não recomendado agora; o texto cobre o uso e dá notificação legível.

Mini app do Telegram foi descartado: exige URL pública em HTTPS, e o painel roda local/VPS fechada.

Opcional barato: uma linha "Em conta: R$ X" no topo do resumo semanal, reaproveitando o mesmo serviço.

## 6. Arquitetura

### 6.1 Núcleo compartilhado

`usecase.Balances` (em `Insights`) lê contas e instituições e devolve uma estrutura de domínio: total, bancos (cada um com contas e cartões já calculados), horário e flags de frescor. Dois consumidores:

- API: serializa para JSON;
- Telegram: `FormatBalances(...)`, função pura em texto (como `FormatDigest`).

Assim os números nunca divergem entre as duas telas.

### 6.2 API

`GET /api/balances` (autenticada, leitura, sem corpo). Resposta (esboço):

```json
{
  "asOf": "2026-10-02T08:12:00Z",
  "totalInAccount": 12345.67,
  "openInvoices": 2140.00,
  "institutions": [{
    "id": "uuid-opaco",
    "name": "Santander",
    "color": "#ec0000",
    "logo": "/api/institutions/uuid-opaco/logo",
    "updatedAt": "...", "stale": false, "shareOfTotal": 0.65,
    "accounts": [{ "id": "...", "name": "Conta corrente", "last4": "1234", "balance": 8000.00, "autoInvested": null }],
    "cards": [{ "id": "...", "name": "Elite", "brand": "MASTERCARD", "last4": "9012",
                "invoice": 1200.00, "limit": 10000, "available": 5800, "used": 4200, "usedRatio": 0.42,
                "closeDate": "2026-10-03", "dueDate": "2026-10-10", "daysToDue": 8, "minimumPayment": 120.00 }]
  }]
}
```

- `Cache-Control: no-store`. O `item_id` do Pluggy **não** sai da API (id opaco da instituição).
- `GET /api/institutions/{id}/logo`: autenticado; devolve a imagem em cache.
- `POST /api/sync` (decisão 7): dispara a sincronização existente; só JSON/mesma origem, limite de taxa; responde 409 se já houver uma em andamento.

### 6.3 Dados e sincronização

Migration `015`:

- tabela `institutions` (`id` uuid, `item_id` único, `name`, `color` com CHECK `^[0-9a-fA-F]{6}$`, `logo` bytea, `logo_mime` com CHECK de tipos permitidos, `logo_checked_at`);
- em `accounts`: `institution_id`, `brand`, `close_date`, `due_date`, `minimum_payment`, `auto_invested_balance`.

Sincronização: o cliente do Pluggy passa a ler o conector do item (`GET /items/{id}`) e os campos extras de `creditData`/`bankData`. **Nunca** gravar `owner`, `taxNumber` nem número completo da conta (regra já vigente). Contas antigas continuam funcionando: sem instituição, o painel agrupa por `item_id` e usa monograma até o próximo sync.

**Logo:** buscar `connector.imageUrl` uma vez (e renovar a cada ≥ 30 dias), com as defesas da seção 7, guardar no banco e servir da própria origem. Mantém o CSP estrito (`img-src 'self' data:`) e evita o navegador falar com terceiros a cada abertura.

### 6.4 Frontend

Página `Painel`, componentes `BalanceHero`, `ShareBar`, `InstitutionCard`, `CreditCardTile`, `Freshness`, `HideValuesToggle`; lógica pura em `lib/balances.ts` (formatação, severidade, escolha de cores com a checagem de ΔE, texto de vencimento) com testes. Reaproveita `categoryVisual`/tokens de tema; segue as regras de gráfico do projeto (vão de 2 px, legenda sempre, texto em tinta de texto, tabela alternativa).

## 7. Segurança e privacidade

- **Rotas** atrás da sessão do dashboard; leituras sem efeitos; `POST /api/sync` com JSON + mesma origem + limite de taxa, como as demais escritas.
- **Logo (SSRF e conteúdo ativo):** só `https`, resolução de DNS bloqueando IP privado/loopback/link-local, sem seguir redirecionamentos para fora das regras, limite de tamanho (256 KB) e de tempo, tipo aceito só `image/png`, `image/jpeg`, `image/webp` e `image/svg+xml`; **SVG é servido com `Content-Security-Policy: default-src 'none'; sandbox` e `X-Content-Type-Options: nosniff`** (como `<img>` não executa script, e aberto direto também não). Falha na busca nunca derruba o sync: cai no monograma.
- **Cor de marca** vem de um campo validado por regex; entra na página como valor de CSS por propriedade controlada, nunca como texto livre.
- **Dados mínimos:** nada de CPF, titular ou número de conta; API e Telegram usam o nome do banco e os 4 últimos dígitos já existentes.
- **Telegram:** os saldos passam pelos servidores do Telegram, como qualquer mensagem do bot (e como o resumo semanal). O comando só atende o `TELEGRAM_CHAT_ID` configurado, em conversa privada; texto do banco escapado em HTML; nada registrado em log.
- **Ocultar valores** é só do navegador (não protege contra quem tem acesso à sessão).

## 8. Testes e critérios de aceite

**Testes automatizados**
- Núcleo `Balances`: total com saldo negativo, fatias só de saldos positivos, cartão com e sem limite, severidade nos limites exatos (70%, 90%, 3 dias, vencida), frescor (36 h), banco sem conector (agrupa por `item_id`), conta sem cartão, valores nulos.
- Escolha de cores: par que passa, par que reprova (usa a série categórica), contraste do texto sobre a cor.
- SQL: migration aplicada em banco existente; sync idempotente; `institutions` não duplica.
- Sincronização com Pluggy de mentira: conector presente, ausente e com `imageUrl` malicioso (host interno, redirecionamento, tamanho enorme, tipo errado, SVG com script) — nada disso derruba o sync nem é servido.
- API: autenticação, `Cache-Control`, `item_id` ausente da resposta, logo com cabeçalhos de segurança.
- Telegram: formatação (escape de HTML, limite de 4096, "e mais N"), só o dono responde, sem Open Finance.
- Front: formatação, severidade, ΔE, textos de vencimento; tipos e lint.

**Aceite (verificado no navegador, com dados reais)**
1. O total e os valores por banco batem com a tela "Contas e cartões" atual.
2. Cada banco mostra logo (ou monograma), nome, faixa na cor de marca e, dentro dele, conta e cartões.
3. Com Santander e Itaú, as fatias da barra são distinguíveis (a regra de ΔE aplicada) e a legenda não depende de cor.
4. Cartão a ≥ 70% / ≥ 90% e fatura perto do vencimento aparecem com ícone e texto.
5. Tema claro e escuro, celular (≈ 390 px) sem rolagem horizontal, `prefers-reduced-motion` respeitado, navegação por teclado.
6. "Ocultar valores" esconde tudo e persiste ao recarregar.
7. `/saldos` no Telegram mostra os mesmos números do painel, legível na notificação.
8. O CSP do nginx não foi afrouxado.

## 9. Plano de entrega

1. **feat: instituições e dados dos cartões na sincronização** (migration 015, conector do Pluggy, cache seguro do logo, testes).
2. **feat: serviço de saldos** (`Balances` + `FormatBalances`, testes).
3. **feat: API de saldos e logos** (`/api/balances`, `/api/institutions/{id}/logo`, `/api/sync`).
4. **feat: tela Painel** (componentes, cores com ΔE, ocultar valores, estados; README e CHANGELOG).
5. **feat: comando /saldos no Telegram** (T1; README e CHANGELOG).
6. *(opcional)* botões inline (T2) e linha de saldo no resumo semanal.

Cada commit atualiza README e CHANGELOG, traz testes e, quando houver migration, a instrução manual de aplicar.

## 10. Decisões em aberto

| # | Pergunta | Recomendação |
| --- | --- | --- |
| 1 | Mostrar "Faturas em aberto" (soma dos cartões) como linha à parte? | **Sim**, discreta, sem subtrair do total em conta |
| 2 | Somar o "aplicado automaticamente" do Itaú ao total em conta? | **Não**: mostrar sob o banco; ele já está no Patrimônio |
| 3 | O Painel vira a página inicial (`/`)? | **Sim**; a Visão geral passa a segunda do menu |
| 4 | Manter "Contas e cartões"? | **Redirecionar** para o Painel |
| 5 | Logos: cache do Pluggy ou só monograma? | **Os dois**: logo quando houver, monograma como reserva |
| 6 | Telegram: só texto ou imagem? | **Texto** (T1); botões depois; imagem só se faltar |
| 7 | Botão "Sincronizar agora" no painel? | **Sim** (`POST /api/sync`), reaproveitando a sincronização do `/sync` |
| 8 | Sufixo dos cartões na mensagem do Telegram: final do cartão ou só o nome? | **Final** (4 dígitos), como no painel |

## 11. Riscos

- Campos de `creditData` e `bankData` podem vir vazios conforme o banco: todos são opcionais e a tela degrada sem erro.
- O `imageUrl` do Pluggy pode mudar de host ou sumir: monograma de reserva e renovação periódica.
- Marcas registradas: os logos são mostrados só no seu ambiente, vindos do conector; **não** são versionados no repositório.
- O saldo de CREDIT como "valor devido" é o que a documentação diz; conferir com a fatura real do seu banco antes de rotular "Fatura" (no Itaú pode incluir parcelados futuros). Se divergir, o rótulo muda para "Saldo devedor".
- Atualização diária do Meu Pluggy: o painel mostra o horário para ninguém confundir "saldo de ontem" com "saldo de agora".
