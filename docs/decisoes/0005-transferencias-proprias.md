# 0005 — Transferências entre contas próprias são ignoradas

**Contexto.** Pix enviado e recebido entre contas da mesma pessoa aparecia como gasto e como renda (cerca de R$ 5,5 mil em saídas num ano), inflando despesas e a renda da projeção. O Pluggy marca parte deles como `Same person transfer`, mas não todos.

**Decisão.** Além da categoria do Pluggy, Pix, TED e DOC cujo texto contém um dos nomes de `OWN_NAMES` são descartados na sincronização. Ao salvar os nomes, o app procura lançamentos antigos parecidos e **pergunta** antes de cancelá-los (ficam `CANCELLED`, nunca apagados). Nomes com menos de 5 caracteres são recusados para não casar com outras pessoas.

**Consequências.** Pix para parentes com outro nome continua sendo gasto. Quem não define `OWN_NAMES` mantém o comportamento anterior.
