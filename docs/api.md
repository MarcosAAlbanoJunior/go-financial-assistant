# API do dashboard

Com `DASHBOARD_PASSWORD` definida (mínimo de 12 caracteres), o app expõe uma API JSON sob `/api`, **somente leitura** (as únicas escritas, além do login, são a correção manual de contas fixas, dispensar sugestões da revisão criar ou apagar metas e o histórico do Coach (análises e respostas)), que alimenta o front-end. Sem a variável, a API nem é montada.

- **Autenticação:** `POST /api/login` com `{"password": "..."}` (`Content-Type: application/json`) devolve um cookie de sessão `HttpOnly`, `SameSite=Strict` (e `Secure` atrás de HTTPS) válido por 7 dias. Reiniciar o app encerra as sessões. Todas as outras rotas respondem `401` sem sessão. O login é limitado a 5 tentativas por minuto por IP.
- **Proteção contra CSRF:** o login e o logout exigem JSON e recusam requisições cujo `Origin` não seja o próprio host, além do `SameSite=Strict`.
- **Rotas** (`month` e `from`/`to` no formato `AAAA-MM`; a janela de `from`/`to` vai de 1 a 60 meses, padrão: últimos 12):

| Rota | Conteúdo |
| --- | --- |
| `POST /api/login`, `POST /api/logout`, `GET /api/me` | sessão |
| `GET /api/summary?month=` | totais do mês e do mês anterior (receitas, despesas, aplicado, resgatado) e o saldo das contas correntes (`bankBalance`, `null` sem Open Finance) |
| `GET /api/timeseries?from=&to=` | os mesmos totais, mês a mês |
| `GET /api/breakdown?month=&by=category\|payment_method\|account` | despesas do mês agrupadas |
| `GET /api/investments?from=&to=` | aplicado, resgatado e **líquido acumulado desde o primeiro lançamento** |
| `GET /api/transactions?month=&kind=&category=&payment_method=&account=&q=&page=&limit=` | lista paginada (padrão 50, máx. 100), manual e Open Finance |
| `GET /api/accounts` | contas e cartões, com saldo e limite |
| `GET /api/balances` | painel: total em conta, bancos, contas e cartões (saldo devedor, limite, vencimento) |
| `GET /api/institutions/{id}/logo` | logo do banco em cache |
| `POST /api/sync` | sincroniza o Open Finance agora (JSON, mesma origem; 409 se já houver uma em andamento) |
| `GET /api/budget?month=` | despesas do mês e dos 11 anteriores por classe (fixas, parceladas, variáveis) e as contas do mês |
| `GET /api/projection?months=` | base da projeção: premissas (renda, fixas, variáveis) e parcelas já conhecidas por mês |
| `PUT /api/expense-rules` | corrige a classe de uma conta (`FIXED`, `VARIABLE` ou `AUTO`); só JSON na mesma origem |
| `GET /api/review?month=` | revisão do mês: matriz categoria × 6 meses e sugestões de corte (aumentos, fixas, gasto formiga, duplicatas, contas novas) com a economia em R$/mês e R$/ano |
| `PUT /api/review-dismissals` | dispensa (ou restaura) uma sugestão da revisão; só JSON na mesma origem |
| `GET /api/categorize` | contas com despesa em "Outros" (da maior para a menor), as categorias atribuíveis e a prévia do que a IA receberia |
| `PUT /api/categorize/rules` | classifica uma conta (ou mantém em Outros): reclassifica as despesas antigas e vale nas próximas sincronizações; só JSON na mesma origem |
| `POST /api/categorize/suggest` | pede à IA categorias para as contas de comércio e serviços da prévia (nunca Pix); não grava nada. Mesmas travas do Coach |
| `GET /api/savings` | economia realizada: as contas que você marcou como canceladas, conferidas mês a mês (cobrança sumiu, voltou ou aguardando), com o total acumulado e o ritmo por mês e por ano |
| `PUT /api/savings/decisions` | marca (ou desfaz) "cancelei" numa sugestão fixa ou de gasto formiga; só JSON na mesma origem |
| `GET /api/goals` | metas com o andamento calculado na hora: patrimônio (contas correntes + investimentos), projeção e gastos por categoria |
| `POST /api/goals`, `DELETE /api/goals/{id}` | cria (juntar valor até uma data, reduzir uma categoria, reserva de N meses; máx. 20) ou apaga uma meta; só JSON na mesma origem |
| `GET /api/coach/preview?month=` | o que o Coach enviaria ao Gemini (mesmo JSON, com hash), sem enviar nada |
| `POST /api/coach/analyze` | envia o que a prévia mostrou (o hash precisa bater) e devolve a análise da IA validada; só JSON na mesma origem, 3/min por IP, uma por vez |
| `GET /api/coach/analyses?month=` | análises guardadas do mês, com as respostas que você deu |
| `PUT /api/coach/analyses/{id}/answers`, `DELETE /api/coach/analyses/{id}` | grava (ou apaga, se vazia) a resposta a uma pergunta da IA; apaga a análise. Só JSON na mesma origem |
| `GET /api/portfolio` | posições de investimento (saldo real do Open Finance), total e total por tipo |
| `GET /api/portfolio/history?from=&to=` | saldo total ao fim de cada mês (existe a partir da primeira sincronização) |

- **Segurança:** a porta `3000` do app é publicada só em `127.0.0.1`. Para acessar de outra máquina, ponha um proxy com **HTTPS** na frente (sem HTTPS a senha e o cookie trafegam em claro) e repasse `X-Real-IP` e `X-Forwarded-Proto`, usados pelo rate limit e pelo atributo `Secure` do cookie; esses cabeçalhos só são aceitos de IPs da rede privada.

### Configurações (exigem `DASHBOARD_PASSWORD` e a migration `016`/`017`)

| Rota | Conteúdo |
| --- | --- |
| `GET /api/settings` | configurações do canal ativo, com a origem do valor (salvo, ambiente ou padrão); segredos nunca voltam, só "configurado" |
| `PUT /api/settings` | salva um lote (tudo ou nada); mudar o que é sensível exige `password` (a senha do dashboard). Ao mudar `OWN_NAMES`, a resposta traz os Pix/TED antigos parecidos |
| `POST /api/settings/reset/{key}` | volta ao valor do ambiente/padrão (sensíveis exigem `password`) |
| `GET /api/settings/audit` | últimas alterações (sem valores) |
| `POST /api/settings/own-transfers/apply` | cancela as transferências entre contas suas encontradas |
| `POST /api/settings/test/{pluggy\|gemini\|telegram}` | testa a conexão com as credenciais salvas |
| `POST /api/restart` | pede ao processo que reinicie (o Docker o sobe de novo) |

