import {
  createContext,
  useContext,
  useState,
  useCallback,
  type ReactNode,
} from "react"
import { X, CheckCircle, AlertCircle, Info, AlertTriangle } from "lucide-react"
import { cn } from "@/lib/utils"

export type ToastType = "success" | "error" | "warning" | "info"

export interface Toast {
  id: string
  type: ToastType
  message: string
}

interface ToastContextValue {
  toast: {
    success: (message: string) => void
    error: (message: string) => void
    warning: (message: string) => void
    info: (message: string) => void
  }
}

const ToastContext = createContext<ToastContextValue | undefined>(undefined)

const icons = {
  success: CheckCircle,
  error: AlertCircle,
  warning: AlertTriangle,
  info: Info,
}

const styles = {
  success: "border-terracotta-200 bg-terracotta-50 text-terracotta-800",
  error: "border-red-200 bg-red-50 text-red-800",
  warning: "border-amber-200 bg-amber-50 text-amber-800",
  info: "border-slate-200 bg-slate-50 text-slate-800",
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  const addToast = useCallback((type: ToastType, message: string) => {
    const id = Math.random().toString(36).slice(2)
    setToasts((prev) => [...prev, { id, type, message }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, 4000)
  }, [])

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  const value: ToastContextValue = {
    toast: {
      success: (msg) => addToast("success", msg),
      error: (msg) => addToast("error", msg),
      warning: (msg) => addToast("warning", msg),
      info: (msg) => addToast("info", msg),
    },
  }

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="fixed right-4 top-4 z-[100] flex flex-col gap-2 max-md:bottom-[calc(4.75rem+env(safe-area-inset-bottom))] max-md:left-3 max-md:right-3 max-md:top-auto">
        {toasts.map((t) => {
          const Icon = icons[t.type]
          return (
            <div
              key={t.id}
              className={cn(
                "flex w-80 max-w-full items-start gap-3 rounded-lg border p-4 shadow-lg animate-slide-in-right",
                styles[t.type]
              )}
            >
              <Icon className="mt-0.5 h-4 w-4 shrink-0" />
              <p className="flex-1 text-sm font-medium">{t.message}</p>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => navigator.clipboard.writeText(t.message)}
                  className="text-xs opacity-60 hover:opacity-100"
                >
                  Copy
                </button>
                <button
                  onClick={() => removeToast(t.id)}
                  className="opacity-60 hover:opacity-100"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
              <div className="absolute bottom-0 left-0 h-0.5 w-full overflow-hidden rounded-b-lg">
                <div
                  className={cn(
                    "h-full animate-[shrink_4s_linear_forwards]",
                    t.type === "success" && "bg-terracotta-400",
                    t.type === "error" && "bg-red-400",
                    t.type === "warning" && "bg-amber-400",
                    t.type === "info" && "bg-slate-400"
                  )}
                  style={{
                    animation: "shrink 4s linear forwards",
                  }}
                />
              </div>
            </div>
          )
        })}
      </div>
      <style>{`
        @keyframes shrink {
          from { width: 100%; }
          to { width: 0%; }
        }
      `}</style>
    </ToastContext.Provider>
  )
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error("useToast must be used within ToastProvider")
  return ctx.toast
}
