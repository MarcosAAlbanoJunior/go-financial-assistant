import { useBreakdown } from '../api/client'
import type { BreakdownBy } from '../api/types'
import { MonthFilter } from '../components/MonthFilter'
import { QueryState } from '../components/QueryState'
import { RankedBars } from '../components/RankedBars'
import { formatMonthTitle } from '../lib/format'
import { useMonth } from '../lib/useMonth'

const SECTIONS: { by: BreakdownBy; title: string }[] = [
  { by: 'category', title: 'Por categoria' },
  { by: 'payment_method', title: 'Por forma de pagamento' },
  { by: 'account', title: 'Por conta ou cartão' },
]

export default function Spending() {
  const { month, setMonth, now } = useMonth()
  return (
    <>
      <h1 className="page-title">Gastos de {formatMonthTitle(month)}</h1>
      <MonthFilter month={month} now={now} onChange={setMonth} />
      <div className="grid-2">
        {SECTIONS.map((s) => (
          <Section key={s.by} month={month} {...s} />
        ))}
      </div>
    </>
  )
}

function Section({ month, by, title }: { month: string; by: BreakdownBy; title: string }) {
  const query = useBreakdown(month, by)
  return (
    <section className="card" aria-labelledby={`h-${by}`}>
      <h2 className="chart-title" id={`h-${by}`}>
        {title}
      </h2>
      <QueryState query={query}>{(items) => <RankedBars items={items} stale={query.isPlaceholderData} categories={by === 'category'} />}</QueryState>
    </section>
  )
}
