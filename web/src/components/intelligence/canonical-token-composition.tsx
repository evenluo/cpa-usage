import { ACCOUNTING_STATE_LABELS } from "@/features/usage-intelligence/view-model"
import type { AccountingState, AccountingSummary } from "@/types/api"

export function CanonicalTokenComposition({ accounting }: { accounting: AccountingSummary }) {
  const hasComposition = accounting.valid_attempts > 0
  const { composition } = accounting
  const excluded = (Object.keys(accounting.states) as AccountingState[])
    .filter((state) => state !== "valid" && accounting.states[state] > 0)

  return (
    <details className="min-w-0 rounded-lg border border-border bg-card px-4 py-3 text-xs">
      <summary className="cursor-pointer rounded-sm font-medium text-foreground transition-colors hover:text-terracotta-700 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring dark:hover:text-terracotta-300">
        Token breakdown
      </summary>
      <div className="mt-3 space-y-4 border-t border-border pt-3">
        <p className="text-muted-foreground">
          {`${accounting.valid_attempts.toLocaleString("en")} of ${accounting.total_attempts.toLocaleString("en")} attempts · ${accounting.valid_quality.complete.toLocaleString("en")} complete · ${(accounting.valid_quality.inconsistent + accounting.valid_quality.unclassified).toLocaleString("en")} with gaps`}
          {excluded.length > 0
            ? ` · excluded: ${excluded.map((state) => `${state === "absent" ? "missing token data" : ACCOUNTING_STATE_LABELS[state]} ${accounting.states[state].toLocaleString("en")}`).join("; ")}`
            : ""}
        </p>
        {hasComposition ? (
          <>
            <dl className="grid grid-cols-2 gap-3 sm:max-w-sm">
              <div>
                <dt className="text-muted-foreground">Total tokens</dt>
                <dd className="font-medium tabular-nums">{composition.total_tokens.toLocaleString("en")}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Unclassified tokens</dt>
                <dd className="font-medium tabular-nums">{composition.unclassified_tokens.toLocaleString("en")}</dd>
              </div>
            </dl>
            <div className="grid min-w-0 gap-3 sm:grid-cols-2">
              <TokenCompositionBar label="Input" total={composition.input.total_tokens} segments={[
                { label: "Uncached", value: composition.input.uncached_tokens, color: "bg-blue-500" },
                { label: "Cache read", value: composition.input.cache_read_tokens, color: "bg-emerald-500" },
                { label: "Cache write", value: composition.input.cache_write_tokens, color: "bg-amber-500" },
              ]} />
              <TokenCompositionBar label="Output" total={composition.output.total_tokens} segments={[
                { label: "Non-reasoning", value: composition.output.non_reasoning_tokens, color: "bg-blue-500" },
                { label: "Reasoning", value: composition.output.reasoning_tokens, color: "bg-violet-500" },
              ]} />
            </div>
          </>
        ) : (
          <p className="text-muted-foreground">Token totals unavailable. Historical token details are missing.</p>
        )}
        <p className="text-muted-foreground">Not a provider invoice.</p>
      </div>
    </details>
  )
}

function TokenCompositionBar({ label, total, segments }: {
  label: string
  total: number
  segments: Array<{ label: string; value: number; color: string }>
}) {
  return (
    <section aria-label={`${label} composition`} className="min-w-0 space-y-2 rounded-md bg-muted/30 p-3">
      <p className="flex flex-wrap justify-between gap-x-2 font-medium">
        <span>{label}</span><span className="tabular-nums">{total.toLocaleString("en")} tokens</span>
      </p>
      {total > 0 ? (
        <div className="flex h-2 overflow-hidden rounded-full bg-muted" aria-hidden="true">
          {segments.map((segment) => (
            <span key={segment.label} className={segment.color} style={{ width: `${segment.value / total * 100}%` }} />
          ))}
        </div>
      ) : null}
      <dl className="space-y-1.5">
        {segments.map((segment) => {
          const share = total > 0 ? segment.value / total * 100 : null
          const shareLabel = share === null ? null : share > 0 && share < 0.1 ? "<0.1%" : `${share.toFixed(1)}%`
          return (
            <div key={segment.label} className="flex flex-wrap justify-between gap-x-2 gap-y-0.5">
              <dt className="flex min-w-0 items-center gap-1.5 text-muted-foreground">
                <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${segment.color}`} aria-hidden="true" />{segment.label}
              </dt>
              <dd className="font-medium tabular-nums">
                {segment.value.toLocaleString("en")}{shareLabel === null ? null : <span className="text-muted-foreground"> · {shareLabel}</span>}
              </dd>
            </div>
          )
        })}
      </dl>
    </section>
  )
}
