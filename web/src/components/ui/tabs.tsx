import * as React from 'react'

import { cn } from '@/lib/utils'

export function Tabs({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('flex items-center gap-1 rounded-full border border-border bg-muted p-1', className)} {...props} />
}

export function TabsTrigger({ className, 'aria-selected': selected, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      aria-selected={selected}
      className={cn(
        'rounded-full px-3 py-1.5 text-xs font-semibold text-muted-foreground transition-colors',
        selected && 'bg-card text-foreground shadow-sm',
        className,
      )}
      type="button"
      {...props}
    />
  )
}
