import { useQuery } from "@tanstack/react-query"
import { apiFetch } from "@/lib/api"

interface AuthSession {
  authenticated: boolean
}

async function fetchSession(): Promise<AuthSession> {
  try {
    return await apiFetch<AuthSession>("/auth/session")
  } catch {
    return { authenticated: false }
  }
}

export function useAuth() {
  return useQuery({
    queryKey: ["auth", "session"],
    queryFn: fetchSession,
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
}
