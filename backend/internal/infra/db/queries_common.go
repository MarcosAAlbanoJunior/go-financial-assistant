package db

import (
	"strings"
)

// paymentMonth é o mês a que um pagamento pertence, igual ao usado nas demais consultas.
// txDate é o dia da transação (vencimento, mês de referência ou criação).
const txDate = `COALESCE(pay.due_date, pay.reference_month, pay.created_at::date)`

const paymentMonth = `DATE_TRUNC('month', COALESCE(pay.due_date, pay.reference_month, pay.created_at))::date`

// monthlyCTE agrega por mês as entradas, despesas e transferências (pagamentos cancelados ficam de fora).
const monthlyCTE = `
	monthly AS (
		SELECT ` + paymentMonth + ` AS month,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'INCOME'), 0)  AS income,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'EXPENSE'), 0) AS expense,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'TRANSFER' AND p.transfer_direction = 'OUT'), 0) AS applied,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'TRANSFER' AND p.transfer_direction = 'IN'), 0)  AS redeemed
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE pay.status != 'CANCELLED'
		GROUP BY 1
	)`

// escapeLike neutraliza os curingas do LIKE para o texto buscado valer literalmente.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// expenseKeyBase normaliza a descrição: minúsculas, só letras (some número, data, parcela e pontuação) e espaços compactados.
const expenseKeyBase = `BTRIM(REGEXP_REPLACE(REGEXP_REPLACE(LOWER(p.description), '[^a-zà-ÿ ]+', ' ', 'g'), '\s+', ' ', 'g'))`

// debitPrefix é o início das compras no débito do Itaú ("DEBITO VISA ELECTRON BRASIL 20/09 NETFLIX..."): esconde o
// comércio, que ainda chega com grafias diferentes a cada mês (NETFLIX.COM, NETFLIX ENTRETENIME).
const debitPrefix = `debito visa electron brasil`

// expenseKey reconhece a mesma conta em meses diferentes. Nas compras no débito do Itaú, a chave é o prefixo mais a primeira
// palavra do comércio, para as grafias do mesmo comércio caírem na mesma conta.
const expenseKey = `(CASE WHEN ` + expenseKeyBase + ` ~ '^` + debitPrefix + ` ' THEN 'debito ' || SPLIT_PART(BTRIM(REGEXP_REPLACE(` + expenseKeyBase + `, '^` + debitPrefix + `', '')), ' ', 1) ELSE ` + expenseKeyBase + ` END)`

// cleanDescription tira o prefixo e a data das compras no débito do Itaú, para o nome mostrado ser o do comércio.
const cleanDescription = `REGEXP_REPLACE(p.description, '^DEBITO VISA ELECTRON BRASIL +[0-9]{2}/[0-9]{2} +', '', 'i')`
