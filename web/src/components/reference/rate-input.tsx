export interface RateInputProps {
  label: string
  value: string
  onChange: (value: string) => void
}

export function RateInput({
  label,
  value,
  onChange,
}: RateInputProps) {
  return (
    <label className="space-y-1">
      <span className="text-[10px] font-medium text-muted-foreground">{label}</span>
      <input
        type="number"
        min="0"
        step="0.000001"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder="-"
        className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none focus-visible:ring-1 focus-visible:ring-terracotta-500"
      />
    </label>
  )
}
