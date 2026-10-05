import { STEPS, stepNumber, type Step } from '../lib/setup'

export function Progress({ step }: { step: Step }) {
  const n = stepNumber(step)
  return (
    <div className="setup-progress">
      <p className="setup-progress-text">
        Passo {n} de {STEPS.length}: {STEPS[n - 1].label}
      </p>
      <ol className="setup-steps" aria-hidden="true">
        {STEPS.map((s, i) => (
          <li key={s.id} className={i + 1 < n ? 'is-done' : i + 1 === n ? 'is-current' : ''} />
        ))}
      </ol>
    </div>
  )
}
