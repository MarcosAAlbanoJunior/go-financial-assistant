# 0003 — Segredos cifrados, chave mestra fora do banco

**Contexto.** A página de Configurações permite salvar tokens e chaves de API no navegador. O banco é copiado em backups; o `.env` não.

**Decisão.** Segredos são cifrados com AES-256-GCM antes de ir ao banco, presos ao nome da chave (um segredo copiado para outra chave não decifra). A chave mestra (`APP_SECRET_KEY` ou, melhor, `APP_SECRET_KEY_FILE` apontando para um arquivo 600 montado em `/run/secrets`) existe só no ambiente. A API nunca devolve o valor, só "configurado". Mudar ou restaurar o que é sensível exige uma confirmação (um código novo no chat ou, sem segundo fator, a senha do dashboard de novo; veja a [0007](0007-segundo-fator-no-chat.md)), é registrado num histórico (sem valores) e avisa no chat.

**Consequências.** Perder a chave mestra obriga a digitar os segredos de novo. Sem chave, a página bloqueia só os campos de segredo. Host comprometido continua sendo um risco que o app não resolve.
