import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"

export interface AuthSession {
  authenticated: boolean
}

const AUTH_SESSION_QUERY_KEY = ["auth", "session"] as const

export async function fetchSession(): Promise<AuthSession> {
  const response = await apiFetch<unknown>("/auth/session")
  if (
    typeof response !== "object" ||
    response === null ||
    typeof (response as { authenticated?: unknown }).authenticated !== "boolean"
  ) {
    throw new Error("Session response is malformed")
  }
  return { authenticated: (response as { authenticated: boolean }).authenticated }
}

export function useAuth() {
  return useQuery({
    queryKey: AUTH_SESSION_QUERY_KEY,
    queryFn: fetchSession,
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => apiFetch("/auth/logout", { method: "POST" }),
    onSuccess: () => {
      qc.setQueryData(AUTH_SESSION_QUERY_KEY, { authenticated: false })
      qc.invalidateQueries({ queryKey: AUTH_SESSION_QUERY_KEY })
    },
  })
}
