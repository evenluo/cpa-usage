import anthropicIconUrl from "@/assets/provider-icons/anthropic.png"
import antigravityIconUrl from "@/assets/provider-icons/antigravity.png"
import geminiIconUrl from "@/assets/provider-icons/gemini.svg"
import kimiIconUrl from "@/assets/provider-icons/kimi.svg"
import openAiMarkUrl from "@/assets/provider-icons/openai-mark.svg"
import type { ProviderKind } from "@/features/usage-intelligence/live-capacity"
import { cn } from "@/lib/utils"

const PROVIDER_ICON_URLS: Partial<Record<ProviderKind, string>> = {
  antigravity: antigravityIconUrl,
  claude: anthropicIconUrl,
  codex: openAiMarkUrl,
  "gemini-cli": geminiIconUrl,
  kimi: kimiIconUrl,
}

export function ProviderBrandIcon({
  providerKind,
  label,
  className,
}: {
  providerKind: ProviderKind
  label: string
  className?: string
}) {
  const iconUrl = PROVIDER_ICON_URLS[providerKind]
  if (!iconUrl) {
    return (
      <span
        className={cn("flex h-5 w-5 shrink-0 items-center justify-center rounded-sm text-[9px] font-semibold leading-none text-muted-foreground dark:text-background", className)}
        title={label}
      >
        {providerFallbackMark(label)}
      </span>
    )
  }

  return (
    <img
      src={iconUrl}
      alt=""
      aria-hidden="true"
      className={cn("h-4 w-4 shrink-0 object-contain", className)}
      title={label}
    />
  )
}

function providerFallbackMark(label: string): string {
  const words = label
    .trim()
    .split(/[\s_-]+/)
    .filter(Boolean)

  if (words.length >= 2) {
    return words
      .slice(0, 2)
      .map((word) => word.charAt(0))
      .join("")
      .toUpperCase()
  }

  const compact = words[0] ?? "?"
  const uppercaseLetters = compact.match(/[A-Z]/g)
  if (uppercaseLetters && uppercaseLetters.length >= 2) {
    return uppercaseLetters.slice(0, 2).join("")
  }
  return compact.slice(0, 2).toUpperCase() || "?"
}
