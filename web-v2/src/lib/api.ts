export function appBasePath(): string {
  const value = window.__APP_BASE_PATH__
  if (!value || value === "__APP_BASE_PATH__" || value === "/") {
    return ""
  }
  return value.endsWith("/") ? value.slice(0, -1) : value
}

function withBasePath(path: string): string {
  return `${appBasePath()}${path}`
}

export function apiPath(path: string): string {
  return withBasePath(`/api/v1${path}`)
}

export async function apiFetch<T>(
  path: string,
  options?: RequestInit
): Promise<T> {
  const response = await fetch(apiPath(path), {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  })

  if (!response.ok) {
    const text = await response.text().catch(() => "Unknown error")
    throw new ApiError(response.status, text)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json() as Promise<T>
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public body: string
  ) {
    super(`API error ${status}: ${body}`)
    this.name = "ApiError"
  }
}
