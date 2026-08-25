import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { apiFetch, apiPath, appBasePath, ApiError } from "./api"

const CPA_STORAGE_PREFIX = "enc::v1::"
const CPA_STORAGE_SEED = "cli-proxy-api-webui::secure-storage"

function encodeCPAStorageValue(plaintext: string): string {
  const key = new TextEncoder().encode(
    `${CPA_STORAGE_SEED}|${window.location.host}|${window.navigator.userAgent}`,
  )
  const encrypted = Uint8Array.from(new TextEncoder().encode(plaintext), (byte, index) => byte ^ key[index % key.length])
  return `${CPA_STORAGE_PREFIX}${window.btoa(String.fromCharCode(...encrypted))}`
}

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  })
}

function requestHeaders(call: unknown[] | undefined): Headers {
  const init = (call?.[1] ?? {}) as RequestInit
  return new Headers(init.headers)
}

describe("appBasePath", () => {
  afterEach(() => {
    delete window.__APP_BASE_PATH__
  })

  it("treats a missing, empty, placeholder, or root value as no prefix", () => {
    delete window.__APP_BASE_PATH__
    expect(appBasePath()).toBe("")

    window.__APP_BASE_PATH__ = ""
    expect(appBasePath()).toBe("")

    window.__APP_BASE_PATH__ = "__APP_BASE_PATH__"
    expect(appBasePath()).toBe("")

    window.__APP_BASE_PATH__ = "/"
    expect(appBasePath()).toBe("")
  })

  it("strips a trailing slash from a real prefix and leaves other prefixes intact", () => {
    window.__APP_BASE_PATH__ = "/cpa-usage/"
    expect(appBasePath()).toBe("/cpa-usage")

    window.__APP_BASE_PATH__ = "/cpa-usage"
    expect(appBasePath()).toBe("/cpa-usage")
  })
})

describe("apiPath", () => {
  afterEach(() => {
    delete window.__APP_BASE_PATH__
  })

  it("prefixes /api/v1 and the resolved app base path", () => {
    delete window.__APP_BASE_PATH__
    expect(apiPath("/status")).toBe("/api/v1/status")

    window.__APP_BASE_PATH__ = "/cpa-usage/"
    expect(apiPath("/usage/identities/page")).toBe("/cpa-usage/api/v1/usage/identities/page")
  })
})

describe("apiFetch", () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    window.localStorage.clear()
    delete window.__APP_BASE_PATH__
    vi.stubGlobal("fetch", fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    window.localStorage.clear()
    delete window.__APP_BASE_PATH__
  })

  it("fetches the resolved API path and defaults JSON content type", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ ok: true }))

    await expect(apiFetch<{ ok: boolean }>("/status")).resolves.toEqual({ ok: true })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0][0]).toBe("/api/v1/status")
    expect(requestHeaders(fetchMock.mock.calls[0]).get("Content-Type")).toBe("application/json")
    expect(requestHeaders(fetchMock.mock.calls[0]).has("Authorization")).toBe(false)
  })

  it("honors the app base path when issuing the request", async () => {
    window.__APP_BASE_PATH__ = "/usage/"
    fetchMock.mockResolvedValueOnce(jsonResponse({}))

    await apiFetch("/status")
    expect(fetchMock.mock.calls[0][0]).toBe("/usage/api/v1/status")
  })

  it("injects a Bearer token from an XOR-encoded managementKey JSON string", async () => {
    window.localStorage.setItem("managementKey", encodeCPAStorageValue(JSON.stringify("shared-secret")))
    fetchMock.mockResolvedValueOnce(jsonResponse({}))

    await apiFetch("/status")
    expect(requestHeaders(fetchMock.mock.calls[0]).get("Authorization")).toBe("Bearer shared-secret")
  })

  it("accepts a raw, non-JSON XOR-encoded managementKey", async () => {
    window.localStorage.setItem("managementKey", encodeCPAStorageValue("raw-secret"))
    fetchMock.mockResolvedValueOnce(jsonResponse({}))

    await apiFetch("/status")
    expect(requestHeaders(fetchMock.mock.calls[0]).get("Authorization")).toBe("Bearer raw-secret")
  })

  it("reads a plaintext managementKey that was not XOR-encoded", async () => {
    window.localStorage.setItem("managementKey", JSON.stringify("plain-secret"))
    fetchMock.mockResolvedValueOnce(jsonResponse({}))

    await apiFetch("/status")
    expect(requestHeaders(fetchMock.mock.calls[0]).get("Authorization")).toBe("Bearer plain-secret")
  })

  it("falls back to the persisted cli-proxy-auth management key", async () => {
    window.localStorage.setItem(
      "cli-proxy-auth",
      encodeCPAStorageValue(JSON.stringify({ state: { managementKey: "persisted-secret" } })),
    )
    fetchMock.mockResolvedValueOnce(jsonResponse({}))

    await apiFetch("/status")
    expect(requestHeaders(fetchMock.mock.calls[0]).get("Authorization")).toBe("Bearer persisted-secret")
  })

  it("prefers managementKey over the persisted cli-proxy-auth key", async () => {
    window.localStorage.setItem("managementKey", JSON.stringify("preferred-secret"))
    window.localStorage.setItem(
      "cli-proxy-auth",
      JSON.stringify({ state: { managementKey: "persisted-secret" } }),
    )
    fetchMock.mockResolvedValueOnce(jsonResponse({}))

    await apiFetch("/status")
    expect(requestHeaders(fetchMock.mock.calls[0]).get("Authorization")).toBe("Bearer preferred-secret")
  })

  it("does not inject Authorization when stored keys are blank or not strings", async () => {
    window.localStorage.setItem("managementKey", JSON.stringify("   "))
    window.localStorage.setItem("cli-proxy-auth", JSON.stringify({ state: { managementKey: 12 } }))
    fetchMock.mockResolvedValueOnce(jsonResponse({}))

    await apiFetch("/status")
    expect(requestHeaders(fetchMock.mock.calls[0]).has("Authorization")).toBe(false)
  })

  it("ignores an unreadable XOR payload instead of throwing", async () => {
    window.localStorage.setItem("managementKey", `${CPA_STORAGE_PREFIX}%%%not-base64%%%`)
    fetchMock.mockResolvedValueOnce(jsonResponse({}))

    await expect(apiFetch("/status")).resolves.toEqual({})
    expect(requestHeaders(fetchMock.mock.calls[0]).has("Authorization")).toBe(false)
  })

  it("does not overwrite an explicit Authorization or Content-Type header", async () => {
    window.localStorage.setItem("managementKey", JSON.stringify("shared-secret"))
    fetchMock.mockResolvedValueOnce(jsonResponse({}))

    await apiFetch("/status", {
      headers: {
        Authorization: "Bearer caller-token",
        "Content-Type": "text/plain",
      },
    })

    const headers = requestHeaders(fetchMock.mock.calls[0])
    expect(headers.get("Authorization")).toBe("Bearer caller-token")
    expect(headers.get("Content-Type")).toBe("text/plain")
  })

  it("throws ApiError with the response body for a non-2xx status", async () => {
    fetchMock.mockResolvedValueOnce(new Response("quota exceeded", { status: 403 }))

    const error = await apiFetch("/status").catch((err: unknown) => err)
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ status: 403, body: "quota exceeded", name: "ApiError" })
    expect((error as Error).message).toBe("API error 403: quota exceeded")
  })

  it("uses a fallback body when reading a failed response throws", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 502,
      text: () => Promise.reject(new Error("stream closed")),
      json: () => Promise.reject(new Error("should not parse JSON on failure")),
    })

    const error = await apiFetch("/status").catch((err: unknown) => err)
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ status: 502, body: "Unknown error" })
  })

  it("returns undefined for 204 and does not parse a body", async () => {
    const json = vi.fn(() => Promise.reject(new Error("204 must not be parsed as JSON")))
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 204,
      text: () => Promise.resolve(""),
      json,
    })

    await expect(apiFetch("/usage/identities/1/alias")).resolves.toBeUndefined()
    expect(json).not.toHaveBeenCalled()
  })

  it("propagates JSON parse failures on a successful response", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.reject(new SyntaxError("Unexpected token <")),
    })

    await expect(apiFetch("/status")).rejects.toThrow(SyntaxError)
  })

  it("propagates a network failure without wrapping it as ApiError", async () => {
    fetchMock.mockRejectedValueOnce(new TypeError("Failed to fetch"))

    const error = await apiFetch("/status").catch((err: unknown) => err)
    expect(error).toBeInstanceOf(TypeError)
    expect(error).not.toBeInstanceOf(ApiError)
    expect((error as Error).message).toBe("Failed to fetch")
  })
})
