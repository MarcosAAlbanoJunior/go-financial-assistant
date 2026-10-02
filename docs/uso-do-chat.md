# Uso pelo chat

Com o container rodando, envie mensagens para **si mesmo** no WhatsApp ou para o seu bot no Telegram. Os exemplos abaixo valem para os dois canais.

### Registrar gasto simples
```
gastei 45 reais no almoço no pix
```

### Registrar compra parcelada
```
comprei um tênis de 300 reais em 3x no cartão
```

### Registrar despesa recorrente
```
netflix 55 reais todo mês todo dia 15
```

### Cancelar recorrente
```
cancelar netflix
```

### Registrar entrada (salário, renda)
```
recebi 6000 reais de salário
entrou 500 reais de freela no pix
```

### Registrar transferência entre contas próprias
```
coloquei 2000 reais no cofrinho
resgatei 500 do CDB
```
Transferências não afetam despesas nem entradas — servem apenas para rastrear movimentações entre suas próprias contas.

### Consultar resumo do mês
```
quanto gastei esse mês?
quanto gastei em fevereiro?
```

O resumo mostra:
- **Despesas** por categoria
- **Entradas** totais (se houver)
- **Resultado** do mês (entradas − despesas)
- **Investimentos no mês**: quanto foi aplicado, quanto foi resgatado
- **Em conta**: resultado descontando o líquido que ficou investido

### Importar extrato bancário (PDF)

Envie o PDF do extrato do seu banco diretamente no WhatsApp. O assistente processa todas as transações automaticamente:

- Despesas, entradas e transferências são classificadas pela IA
- Aplicações no cofrinho e resgates de CDB são detectados como **Transferência** — não inflam as despesas
- Transações já existentes no banco são sinalizadas para confirmação individual
- Suporta o formato de extrato do **Itaú** (e outros formatos com datas DD/MM/AAAA ou AAAA-MM-DD)

### Exportar planilha CSV

Peça ao assistente para exportar os gastos de um mês e ele enviará um arquivo `.csv` diretamente na conversa — pronto para abrir no Excel ou Google Sheets:

```
exportar meus gastos de março
me manda o csv de fevereiro 2024
quero a planilha de janeiro
exportar
```

- Se nenhum mês for especificado, exporta o **mês atual**.
- Se não houver lançamentos no período, o assistente avisa por texto.
- O arquivo vem com **BOM UTF-8** para compatibilidade com Excel.
- Colunas: Data, Descrição, Categoria, Forma de Pagamento, Tipo, Parcela, Valor (R$).
- Linhas de totais ao final: **TOTAL DESPESAS**, **TOTAL ENTRADAS** (se houver), **SALDO**, **TOTAL APLICADO** / **TOTAL RESGATADO** (se houver transferências).
- A mensagem que acompanha o arquivo já traz o resumo financeiro: despesas, entradas, resultado, aplicado/resgatado e valor em conta.

> O Gemini interpreta a intenção de exportação, então frases naturais como _"quero ver meus gastos em planilha"_ ou _"gera um csv pra mim"_ também funcionam.

### Relatório mensal automático

No primeiro dia de cada mês, o assistente envia automaticamente a planilha CSV com todos os gastos do mês anterior — sem você precisar pedir.

### Enviar recibo ou nota fiscal
Tire uma foto ou encaminhe a imagem do recibo diretamente na conversa. No Telegram, imagens enviadas "como arquivo" também são tratadas como recibo.
