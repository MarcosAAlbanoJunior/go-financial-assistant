# 0002 — Parcela datada no mês da fatura

**Contexto.** O Pluggy entrega todas as parcelas de uma compra com a data da compra. Uma compra em 12x somava o total no mês da compra e deixava os 11 meses seguintes sem a parcela, distorcendo gastos por mês, revisão, orçamento e projeção.

**Decisão.** Cada parcela é datada no mês de `billForecastDate` (a fatura em que cai), mantendo o dia da compra; sem essa informação, mantém a data da compra. A sincronização corrige parcelas já gravadas quando as reconhece de novo.

**Consequências.** Existem lançamentos futuros pendentes no banco (parcelas ainda não cobradas); as telas só navegam até o mês atual. Compras antigas além de `SYNC_LOOKBACK_DAYS` só são corrigidas com uma sincronização de janela maior (até 365).
