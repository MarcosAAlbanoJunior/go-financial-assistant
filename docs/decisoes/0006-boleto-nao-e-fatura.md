# 0006 — Boleto não é pagamento de fatura só pela categoria

**Contexto.** O Pluggy classifica todo boleto pago (e até Pix QR Code de parcelamento) como `Credit card payment`. O app descartava tudo com essa categoria, e 12 parcelas de R$ 985,83 de um financiamento nunca apareceram.

**Decisão.** A categoria só descarta quando a descrição também parece fatura de cartão (fatura, cartão, bandeira, "pagamento recebido"); Pix nunca é descartado por essa regra.

**Consequências.** É uma heurística por palavras: um boleto de fatura sem nenhuma dessas palavras entraria como despesa (cancelável). Não apareceu nos dados reais.
