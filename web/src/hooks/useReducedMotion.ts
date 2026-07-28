import { useSyncExternalStore } from "react"

const reducedMotionQuery = "(prefers-reduced-motion: reduce)"

function subscribe(onStoreChange: () => void): () => void {
  const media = window.matchMedia(reducedMotionQuery)
  media.addEventListener("change", onStoreChange)
  return () => media.removeEventListener("change", onStoreChange)
}

function getSnapshot(): boolean {
  return window.matchMedia(reducedMotionQuery).matches
}

export function useReducedMotion(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, () => true)
}
