import { createLazyFileRoute, useNavigate } from "@tanstack/react-router"
import { useState, type FormEvent } from "react"
import { apiFetch } from "@/lib/api"
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
  const toast = useToast()

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    try {
      await apiFetch("/auth/login", {
        method: "POST",
        body: JSON.stringify({ password }),
      })
      toast.success("Signed in successfully")
      navigate({ to: "/" })
    } catch (err) {
      const message = err instanceof Error ? err.message : "Login failed"
      toast.error(message)
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
            Enter your dashboard password to continue
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="text-sm font-medium">Password</label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="mt-1.5 h-10 w-full rounded-lg border border-input bg-background px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-terracotta-500"
                placeholder="Enter password"
                required
              />
            </div>
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
