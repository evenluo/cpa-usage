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

const cpaStoragePrefix = "enc::v1::"
const cpaStorageSeed = "cli-proxy-api-webui::secure-storage"

function readSharedManagementKey(): string | null {
  if (typeof window === "undefined") {
    return null
  }
  return readCPAStorageValue("managementKey") ?? readPersistedCPAManagementKey()
}

function readCPAStorageValue(key: string): string | null {
  try {
    const decoded = readCPAStorageString(key)
    try {
      const parsed = JSON.parse(decoded)
      return typeof parsed === "string" && parsed.trim() ? parsed : null
    } catch {
      return decoded.trim() ? decoded : null
    }
  } catch {
    return null
  }
}

function readPersistedCPAManagementKey(): string | null {
  try {
    const persisted = readCPAStorageString("cli-proxy-auth")
    const parsed = JSON.parse(persisted) as {
      state?: { managementKey?: unknown }
    }
    const value = parsed.state?.managementKey
    return typeof value === "string" && value.trim() ? value : null
  } catch {
    return null
  }
}

function readCPAStorageString(key: string): string {
  const raw = window.localStorage.getItem(key)
  if (!raw) {
    return ""
  }
  return raw.startsWith(cpaStoragePrefix) ? decodeCPAStorageValue(raw) : raw
}

function decodeCPAStorageValue(value: string): string {
  const payload = window.atob(value.slice(cpaStoragePrefix.length))
  const encrypted = Uint8Array.from(payload, (char) => char.charCodeAt(0))
  const key = new TextEncoder().encode(
    `${cpaStorageSeed}|${window.location.host}|${window.navigator.userAgent}`
  )
  const decoded = encrypted.map((byte, index) => byte ^ key[index % key.length])
  return new TextDecoder().decode(decoded)
}

function apiHeaders(options?: RequestInit): Headers {
  const headers = new Headers(options?.headers)
  if (!headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json")
  }
  if (!headers.has("Authorization")) {
    const managementKey = readSharedManagementKey()
    if (managementKey) {
      headers.set("Authorization", `Bearer ${managementKey}`)
    }
  }
  return headers
}

export async function apiFetch<T>(
  path: string,
  options?: RequestInit
): Promise<T> {
  const response = await fetch(apiPath(path), {
    ...options,
    headers: apiHeaders(options),
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
