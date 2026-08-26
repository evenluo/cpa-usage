import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"

interface AuthSession {
  authenticated: boolean
}

const AUTH_SESSION_QUERY_KEY = ["auth", "session"] as const

async function fetchSession(): Promise<AuthSession> {
  try {
    return await apiFetch<AuthSession>("/auth/session")
  } catch {
    return { authenticated: false }
  }
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
