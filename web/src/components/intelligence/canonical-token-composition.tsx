import { ACCOUNTING_STATE_LABELS, getCanonicalTokenFields } from "@/features/usage-intelligence/view-model"
import type { AccountingState, AccountingSummary, CostStatus } from "@/types/api"

export function CanonicalTokenComposition({ accounting, costStatus }: { accounting: AccountingSummary; costStatus: CostStatus }) {
  const hasComposition = accounting.valid_attempts > 0
  const excluded = (Object.keys(accounting.states) as AccountingState[])
    .filter((state) => state !== "valid" && accounting.states[state] > 0)

  return (
    <details className="mt-2 min-w-0 border-t border-border pt-2 text-xs">
      <summary className="cursor-pointer rounded-sm font-medium text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring">
        Canonical token composition
      </summary>
      <div className="mt-3 space-y-3">
        <p className="text-muted-foreground">
          {accounting.valid_attempts.toLocaleString("en")} / {accounting.total_attempts.toLocaleString("en")} valid
          {" · "}complete {accounting.valid_quality.complete.toLocaleString("en")}
          {" · "}inconsistent {accounting.valid_quality.inconsistent.toLocaleString("en")}
          {" · "}unclassified {accounting.valid_quality.unclassified.toLocaleString("en")}
          {excluded.length > 0
            ? ` · excluded: ${excluded.map((state) => `${ACCOUNTING_STATE_LABELS[state]} ${accounting.states[state].toLocaleString("en")}`).join("; ")}`
            : ""}
        </p>
        {hasComposition ? (
          <>
            <dl className="grid grid-cols-2 gap-x-4 gap-y-2">
              {getCanonicalTokenFields(accounting.composition).map(([label, value]) => (
                <div key={label} className="min-w-0">
                  <dt className="text-muted-foreground">{label}</dt>
                  <dd className="break-words font-medium tabular-nums">{value?.toLocaleString("en")}</dd>
                </div>
              ))}
            </dl>
            <p className="text-muted-foreground">Input = uncached + cache read + cache write; output = non-reasoning + reasoning.</p>
          </>
        ) : (
          <p className="text-muted-foreground">Canonical totals unavailable. Missing historical facts cannot be reconstructed from scalar tokens.</p>
        )}
        <p className="text-muted-foreground">Local estimate ({costStatus}); not an upstream billing amount.</p>
      </div>
    </details>
  )
}
