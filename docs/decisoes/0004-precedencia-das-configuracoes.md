# 0004 — Configurações: valor salvo > ambiente > padrão

**Contexto.** Antes, tudo vinha do ambiente (`.env`). A página de Configurações permite ajustar sem editar arquivo nem reiniciar.

**Decisão.** Cada configuração se resolve nesta ordem: valor salvo no dashboard, variável de ambiente, padrão. "Voltar ao valor do ambiente" apaga o salvo. O que muda comportamento em andamento (resumo, sincronização, nomes, Pluggy, plano pago do Coach) vale na hora; canal, chave do Gemini e afins pedem reinício (a página mostra e oferece o botão). Porta, banco, senha do dashboard, canal e backup ficam só no ambiente.

**Consequências.** Um valor salvo que deixe o app inválido nunca impede a subida: `config.Load` cai para só o ambiente. Um token do Telegram errado deixa só o canal desligado, com o dashboard no ar para corrigir.
