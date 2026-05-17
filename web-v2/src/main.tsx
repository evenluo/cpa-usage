import React from "react"
import ReactDOM from "react-dom/client"
import { RouterProvider, createRouter } from "@tanstack/react-router"
import { QueryProvider } from "@/components/providers/query-provider"
import "./index.css"

import { routeTree } from "./routeTree.gen"

const router = createRouter({ routeTree })

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router
  }
}

const rootElement = document.getElementById("root")!
if (!rootElement.innerHTML) {
  const root = ReactDOM.createRoot(rootElement)
  root.render(
    <React.StrictMode>
      <QueryProvider>
        <RouterProvider router={router} />
      </QueryProvider>
    </React.StrictMode>
  )
}
