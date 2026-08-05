import "@testing-library/jest-dom/vitest"

// vitest 4 + jsdom 29 默认不启用 localStorage（除非提供 --localstorage-file）。
// 为依赖浏览器持久化语义的测试（时间范围记忆、主题记忆）提供进程内实现。
if (typeof window !== "undefined" && typeof window.localStorage === "undefined") {
  const store = new Map<string, string>()
  const localStorageMock: Storage = {
    get length() {
      return store.size
    },
    clear: () => {
      store.clear()
    },
    getItem: (key: string) => store.get(key) ?? null,
    key: (index: number) => Array.from(store.keys())[index] ?? null,
    removeItem: (key: string) => {
      store.delete(key)
    },
    setItem: (key: string, value: string) => {
      store.set(key, String(value))
    },
  }
  Object.defineProperty(window, "localStorage", { value: localStorageMock, configurable: true })
}
