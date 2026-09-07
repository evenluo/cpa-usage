import { ACCOUNTING_STATE_LABELS, getAccountingCaption, getCanonicalTokenFields } from "@/features/usage-intelligence/view-model"
import type { AccountingState, AccountingSummary, CostStatus } from "@/types/api"

export function CanonicalTokenComposition({ accounting, costStatus }: { accounting: AccountingSummary; costStatus: CostStatus }) {
  const hasComposition = accounting.valid_attempts > 0
  const excluded = (Object.keys(accounting.states) as AccountingState[])
    .filter((state) => state !== "valid" && accounting.states[state] > 0)

  return (
    <details className="mt-4 min-w-0 border-t border-border pt-3 text-xs">
      <summary className="cursor-pointer rounded-sm text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
        <span className="font-medium text-foreground">Canonical token composition</span>
        <span className="mt-1 block">{getAccountingCaption(accounting)}</span>
      </summary>
      <div className="mt-3 space-y-3">
        <p className="text-muted-foreground">Selected window and provider scope. Every token metric uses this canonical composition.</p>
        <div>
          <h3 className="font-medium">Metric Completeness · Accounting</h3>
          <p className="mt-1 text-muted-foreground">
            {accounting.valid_attempts.toLocaleString("en")} / {accounting.total_attempts.toLocaleString("en")} attempts have valid canonical structure.
            {" "}Coverage counts attempts, not tokens; valid structure does not imply complete quality.
          </p>
          <p className="mt-1 text-muted-foreground">
            Quality among {accounting.valid_attempts.toLocaleString("en")} valid attempts:
            {" "}complete {accounting.valid_quality.complete.toLocaleString("en")},
            {" "}inconsistent {accounting.valid_quality.inconsistent.toLocaleString("en")},
            {" "}unclassified {accounting.valid_quality.unclassified.toLocaleString("en")}.
          </p>
          {excluded.length > 0 ? (
            <p className="mt-1 text-muted-foreground">
              Excluded from composition (of all {accounting.total_attempts.toLocaleString("en")} attempts):
              {" "}{excluded.map((state) => `${ACCOUNTING_STATE_LABELS[state]} ${accounting.states[state].toLocaleString("en")}`).join("; ")}.
            </p>
          ) : null}
        </div>
        {hasComposition ? (
          <>
            <dl className="grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-3">
              {getCanonicalTokenFields(accounting.composition).map(([label, value]) => (
                <div key={label} className="min-w-0">
                  <dt className="text-muted-foreground">{label}</dt>
                  <dd className="break-words font-medium tabular-nums">{value?.toLocaleString("en")}</dd>
                </div>
              ))}
            </dl>
            <p className="text-muted-foreground">Input = uncached + cache read + cache write. Output = non-reasoning + reasoning. Total = input + output + unclassified. Sums include all valid qualities; inconsistent or unclassified quality remains qualified.</p>
          </>
        ) : (
          <p className="text-muted-foreground">Canonical totals unavailable. Missing historical facts cannot be reconstructed from scalar tokens.</p>
        )}
        <p className="text-muted-foreground">Local cost estimate completeness: {costStatus}. Complete canonical rows price uncached and cache-write input at the prompt rate, cache-read input at the cache rate, and total output at the completion rate. This is not an upstream billing amount.</p>
      </div>
    </details>
  )
}
