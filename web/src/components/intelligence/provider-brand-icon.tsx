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
  if (!iconUrl) return null

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
