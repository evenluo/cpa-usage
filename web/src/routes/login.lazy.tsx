import { createLazyFileRoute, useNavigate } from "@tanstack/react-router"
import { useQueryClient } from "@tanstack/react-query"
import { useState, type FormEvent } from "react"
import { ApiError, apiFetch } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useToast } from "@/components/providers/toast-provider"

export const Route = createLazyFileRoute("/login")({
  component: LoginPage,
})

function LoginPage() {
  const [password, setPassword] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const toast = useToast()

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    try {
      await apiFetch("/auth/login", {
        method: "POST",
        body: JSON.stringify({ password }),
      })
      queryClient.setQueryData(["auth", "session"], { authenticated: true })
      toast.success("Signed in")
      navigate({ to: "/" })
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        toast.error("Invalid password")
      } else if (err instanceof ApiError && err.status === 429) {
        toast.error("Too many failed attempts")
      } else {
        toast.error("Login failed")
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-[80vh] items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="font-serif text-2xl">Sign in</CardTitle>
          <CardDescription>
            Dashboard password
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <label className="grid gap-1.5 text-sm font-medium">
              Password
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="h-10 w-full rounded-lg border border-input bg-background px-3 text-sm outline-hidden focus-visible:ring-2 focus-visible:ring-terracotta-500"
                required
              />
            </label>
            <Button
              type="submit"
              className="w-full"
              disabled={submitting || !password.trim()}
            >
              {submitting ? "Signing in..." : "Sign in"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
