# 0001 — Cartão mostra saldo devedor, não fatura

**Contexto.** O Pluggy devolve para o cartão um `balance` que parecia a fatura. Conferindo com o app do banco e com a API (`creditData`), ele é igual a `limite − disponível`: o limite usado, que inclui parcelas futuras e ainda não abate o que foi adiantado. A API `/bills` só traz faturas já fechadas; a fatura aberta não existe nos dados.

**Decisão.** O painel e o `/saldos` rotulam o valor como **saldo devedor** ("inclui parcelas futuras") e nunca como fatura. Não tentamos reconstruir a fatura aberta (a conta feita com as transações do ciclo não bateu com o valor real).

**Consequências.** O valor pode ser maior que a próxima fatura. Se um dia o banco expuser a fatura aberta, ela entra como um campo novo ao lado. Uma alternativa descartada por ora: campo para a pessoa digitar a fatura aberta.
