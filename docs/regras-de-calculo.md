# Regras de cálculo

Tudo é calculado por código (nada de IA nos números). Valores em reais; um mês é identificado pelo seu primeiro dia (UTC).

## O que entra como gasto ou renda (sincronização)

Feito em `infra/pluggy/mapper.go`. A transação do Pluggy vira lançamento, salvo quando é:

- **Transferência entre contas suas**: categoria `Same person transfer` do Pluggy, ou Pix/TED/DOC com um dos nomes de `OWN_NAMES` (`domain.OwnTransferMatcher`). Só move dinheiro de lugar.
- **Pagamento de fatura de cartão**: categoria `Credit card payment` **e** descrição com cara de fatura (fatura, cartão, bandeira, "pagamento recebido"). O Pluggy rotula *todo* boleto pago assim; boleto para um banco (ex.: parcela de financiamento) e Pix QR Code são gasto de verdade.
- **Crédito em cartão** (estorno ou pagamento): não é renda.
- **Aplicação automática do Itaú** ("APLIC AUT MAIS"): varre o saldo para um CDB e de volta; os rendimentos pagos continuam sendo renda.

Aplicações e resgates de investimento viram `TRANSFER` (aparecem em Investimentos, não em gastos). A categoria vem do texto do Pluggy e, quando fica em Outros, de regras por descrição.

### Parcelas no cartão

Cada parcela é datada no **mês da fatura em que cai** (`creditCardMetadata.billForecastDate`), mantendo o dia da compra. O Pluggy dá a mesma data de compra a todas as parcelas; sem o ajuste, as 12 parcelas caíam no mês da compra. Parcelas ainda não cobradas ficam `PENDING`.

### Chave de conta (`expenseKey`)

Para reconhecer a mesma conta em meses diferentes, a descrição é normalizada: minúsculas, só letras (some número, data, parcela, pontuação), espaços compactados. Compras no débito do Itaú ("DEBITO VISA ELECTRON BRASIL dd/mm COMERCIO") usam o prefixo mais a primeira palavra do comércio, porque o nome muda de grafia a cada mês. Regras salvas (fixa/variável, categoria, dispensas, decisões) usam essa chave.

## Orçamento (`planning/budget.go`)

Cada conta (chave) é classificada como **fixa**, **parcelada** ou **variável**, nesta ordem: correção manual, parcelada (`n/m` na descrição ou tipo), recorrente cadastrada, detecção de conta fixa e, por fim, variável. A classe vale para a conta inteira na janela de 12 meses.

**Detecção de fixa:** aparece em pelo menos 3 meses da janela (ou em todos, havendo só 2), no máximo 2 vezes por mês, com valor parecido: (maior − menor) / mediana dos totais mensais ≤ 30% (≤ 5% com só 2 meses de dados, porque o acaso é mais provável).

## Projeção (`planning/projection.go`)

Projeta N meses (6, 12 ou 24) a partir do mês atual:

- **Fixas e variáveis**: média dos últimos 3 meses completos com dados (editável na tela).
- **Renda**: mediana da renda total dos últimos 6 meses (um mês fora do padrão — 13º, adiantamento de férias — não pesa). As fontes recorrentes (`EstimateIncome`) explicam a maior parte; o restante aparece como "outras entradas". **Todo lançamento da categoria Salário é uma fonte só**, mesmo que a descrição mude.
- **Parcelas**: as que faltam das compras parceladas (inferidas do `n/m` da descrição e das já cadastradas).
- **Saldo projetado** = renda − fixas − variáveis − parcelas, acumulado.

É uma estimativa: gastos novos, reajustes e rendimentos não entram.

## Revisão de gastos (`review/review.go`)

Sugestões de corte, sempre com a economia em R$/mês e R$/ano:

| Tipo | Regra |
| --- | --- |
| Aumento | categoria subiu ≥ R$ 50 e ≥ 20% sobre a média dos até 3 meses anteriores |
| Fixa / assinatura | conta fixa detectada; economia anual = valor × 12 |
| Gasto formiga | ≥ 4 compras de até R$ 40 (fora Pix) somando ≥ R$ 80 na mesma conta; economia = metade |
| Possível duplicata | mesmo valor e conta em até 3 dias (≥ R$ 5) |
| Conta nova | só aparece no mês, com ≥ 2 meses anteriores de dados (≥ R$ 30) |

Duplicata e conta nova são avulsas (não anualizadas). Sugestões podem ser **dispensadas** ou marcadas **"Cancelei"** (só fixa e formiga).

### Economia realizada (`review/savings.go`)

Para cada "cancelei", confere mês a mês, do mês seguinte à decisão até hoje, se a cobrança sumiu: **confirmado** (sumiu), **voltou** (o mês trouxe ≥ metade do custo) ou **pendente** (nenhum mês fechado ainda). Economia = custo mensal × meses confirmados. O mês em andamento só serve para flagrar a volta.

## Metas (`planning/goals.go`)

O progresso é calculado na hora. Patrimônio = saldo das contas correntes + investimentos.

- **Juntar valor até uma data**: quanto guardar por mês; "cabe" compara com a sobra média projetada.
- **Reduzir uma categoria em X%**: teto = média dos até 3 meses anteriores à criação (fixada na criação); acompanha cada mês desde então e o ritmo do mês atual (a partir do dia 7).
- **Reserva de N meses**: alvo = N × despesas fixas da projeção; mostra quantos meses o patrimônio cobre.

## Saldos (`balances/`)

- **Total em conta** = soma do saldo das contas `BANK` (negativo entra com sinal). **Participação** do banco = saldo ÷ soma dos saldos positivos (banco negativo não ganha fatia).
- **Cartão** (nunca somado entre cartões): o valor mostrado é o **saldo devedor** informado pelo banco = limite usado (inclui parcelas futuras; não é a fatura aberta, que o Open Finance não expõe). Uso do limite: ≥ 70% atenção, ≥ 90% crítico. Vencimento em até 3 dias: atenção; data já passada não alerta (o banco mantém o vencimento da fatura paga).
- **Desatualizado**: mais de 36 h desde a última atualização *do Pluggy* (`lastUpdatedAt` do item), não desde a última cópia do app. O Meu Pluggy não permite forçar a atualização pela API; ela acontece sozinha (cerca de 1x/dia) ou a pedido em meu.pluggy.ai.
- O "aplicado automaticamente" do banco só aparece se for diferente do saldo.
- O nome e a cor do banco vêm do conector; no Meu Pluggy o conector é sempre o agregador, então vêm do nome das contas (tabela de bancos conhecidos).

## Mês em andamento

Valores do mês atual são parciais. As telas que comparam com o mês anterior avisam, e a Visão geral não mostra a variação enquanto o mês não fecha.

## Investimentos

O saldo vem do Pluggy (`/investments`, líquido de impostos) e é gravado a cada sincronização, formando o histórico do patrimônio. **Do mês da primeira sincronização em diante** o saldo ao fim de cada mês é exato (soma do último saldo gravado de cada posição). **Antes disso é estimado**, por posição: saldo do primeiro registro menos o que foi aplicado depois do mês (e antes desse registro), sem contar rendimentos; posição sem movimentação até o fim do mês ainda não existia e fica de fora; o saldo nunca é negativo. Posições que somem do Pluggy (resgatadas) ficam inativas com saldo zero.

## Conciliação com lançamentos manuais

Ao sincronizar, uma transação do banco que ainda não existe é conciliada com um lançamento manual equivalente (mesmo tipo e valor): avulso, com até 3 dias de diferença; recorrente, no mesmo mês (a recorrência é gerada em dia fixo e o banco pode atrasar). Parcelados ficam de fora porque a data de cada parcela no cartão não é previsível. Se achar, vincula (e marca como pago); senão cria um lançamento novo.
