import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useCallback, useMemo, useState } from "react"
import { apiFetch } from "@/lib/api"
import { collectPaginatedItems } from "@/lib/pagination"
import type {
  KeyIdentity,
  KeyIdentityPage,
  QuotaCheckResponse,
  QuotaObservationsResponse,
  QuotaRefreshResponse,
  QuotaRefreshTaskResponse,
} from "@/types/api"

const PAGE_SIZE = 100
export const QUOTA_REFRESH_LIMIT = 20

export type LiveCapacityTaskState =
  | { status: "starting" }
  | { status: "queued" | "running"; taskId: string }
  | { status: "completed"; taskId: string }
  | { status: "failed"; taskId?: string; error: string }

interface QuotaRefreshBatchResult {
  responses: QuotaRefreshResponse[]
  failedAuthIndexes: Array<{ authIndex: string; error: string }>
}

interface RefreshTaskUpdates {
  updates: Record<string, LiveCapacityTaskState>
  completedObservations: QuotaCheckResponse[]
}

type RefreshTaskPollResult =
  | { authIndex: string; task: QuotaRefreshTaskResponse }
  | { authIndex: string; taskId: string; error: string }

async function fetchAuthFileIdentitiesPage(page: number): Promise<KeyIdentityPage> {
  return apiFetch(`/usage/identities/page?auth_type=1&page=${page}&page_size=${PAGE_SIZE}`)
}

export async function fetchAllAuthFileIdentities(): Promise<KeyIdentity[]> {
  const identities = await collectPaginatedItems<KeyIdentityPage, KeyIdentity>({
    fetchPage: fetchAuthFileIdentitiesPage,
    getItems: (page) => page.identities,
    resource: "Auth-file accounts",
    expectedPageSize: PAGE_SIZE,
  })
  return identities.filter((identity) => identity.auth_type === 1)
}

function identityFingerprint(identities: KeyIdentity[]): string {
  return identities.map((identity) => identity.identity).sort().join("|")
}

export function quotaObservationsQueryKey(provider: string, identities: KeyIdentity[]) {
  return ["quota", "observations", provider || "all", identityFingerprint(identities)] as const
}

async function fetchQuotaObservations(authIndexes: string[]): Promise<QuotaObservationsResponse> {
  return apiFetch("/quota/observations", {
    method: "POST",
    body: JSON.stringify({ auth_indexes: authIndexes, limit: authIndexes.length }),
  })
}

async function refreshQuotaBatch(authIndexes: string[]): Promise<QuotaRefreshResponse> {
  return apiFetch("/quota/refresh", {
    method: "POST",
    body: JSON.stringify({ auth_indexes: authIndexes, limit: QUOTA_REFRESH_LIMIT }),
  })
}

async function fetchRefreshTask(taskId: string): Promise<QuotaRefreshTaskResponse> {
  return apiFetch(`/quota/refresh/${encodeURIComponent(taskId)}`)
}

async function requestQuotaRefresh(authIndexes: string[]): Promise<QuotaRefreshBatchResult> {
  const responses: QuotaRefreshResponse[] = []
  const failedAuthIndexes: QuotaRefreshBatchResult["failedAuthIndexes"] = []
  try {
    responses.push(await refreshQuotaBatch(authIndexes))
  } catch (error) {
    for (const authIndex of authIndexes) {
      failedAuthIndexes.push({ authIndex, error: refreshErrorMessage(error) })
    }
  }
  return { responses, failedAuthIndexes }
}

function matchesProvider(identity: KeyIdentity, provider: string): boolean {
  if (!provider) return true
  return identity.provider.toLowerCase() === provider.toLowerCase()
}

function isActiveTaskState(
  entry: [string, LiveCapacityTaskState],
): entry is [string, Extract<LiveCapacityTaskState, { status: "queued" | "running" }>] {
  return entry[1].status === "queued" || entry[1].status === "running"
}

function activeTaskEntries(taskStates: Record<string, LiveCapacityTaskState>) {
  return Object.entries(taskStates).filter(isActiveTaskState)
}

function activeTaskFingerprint(taskStates: Record<string, LiveCapacityTaskState>): string {
  return activeTaskEntries(taskStates)
    .map(([, state]) => state.taskId)
    .sort()
    .join("|")
}

function hasActiveTasks(taskStates: Record<string, LiveCapacityTaskState>): boolean {
  return activeTaskEntries(taskStates).length > 0
}

function isAnyRefreshState(state: LiveCapacityTaskState): boolean {
  return state.status === "starting" || state.status === "queued" || state.status === "running"
}

export function selectRefreshAuthIndexes(input: {
  requestedAuthIndexes: string[]
  taskStates: Record<string, LiveCapacityTaskState>
  limit?: number
}): string[] {
  const limit = input.limit ?? QUOTA_REFRESH_LIMIT
  return input.requestedAuthIndexes
    .filter((authIndex) => !input.taskStates[authIndex] || !isAnyRefreshState(input.taskStates[authIndex]))
    .slice(0, limit)
}

export async function resolveRefreshTaskUpdates(
  taskStates: Record<string, LiveCapacityTaskState>,
  fetchTask: (taskId: string) => Promise<QuotaRefreshTaskResponse>,
): Promise<RefreshTaskUpdates> {
  const activeTasks = activeTaskEntries(taskStates)
  const results: RefreshTaskPollResult[] = await Promise.all(
    activeTasks.map(async ([authIndex, state]) => {
      try {
        return { authIndex, task: await fetchTask(state.taskId) }
      } catch (error) {
        return { authIndex, taskId: state.taskId, error: refreshErrorMessage(error) }
      }
    }),
  )

  const updates: Record<string, LiveCapacityTaskState> = {}
  const completedObservations: QuotaCheckResponse[] = []
  for (const result of results) {
    if ("error" in result) {
      updates[result.authIndex] = { status: "failed", taskId: result.taskId, error: result.error }
      continue
    }
    const { authIndex, task } = result
    if (task.status === "completed") {
      if (task.quota) {
        updates[authIndex] = { status: "completed", taskId: task.taskId }
        completedObservations.push(task.quota)
      } else {
        updates[authIndex] = {
          status: "failed",
          taskId: task.taskId,
          error: "Refresh completed without an observation",
        }
      }
    } else if (task.status === "failed") {
      updates[authIndex] = { status: "failed", taskId: task.taskId, error: task.error || "failed" }
    } else {
      updates[authIndex] = { status: task.status, taskId: task.taskId }
    }
  }
  return { updates, completedObservations }
}

function observationTime(observation: QuotaCheckResponse): number {
  const parsed = Date.parse(observation.observedAt)
  return Number.isFinite(parsed) ? parsed : Number.NEGATIVE_INFINITY
}

/** Merge observation snapshots by account; a later successful observation is authoritative. */
export function mergeQuotaObservations(
  current: QuotaObservationsResponse | undefined,
  incoming: QuotaObservationsResponse,
): QuotaObservationsResponse {
  const byAuthIndex = new Map((current?.items ?? []).map((item) => [item.id, item]))
  for (const observation of incoming.items) {
    const existing = byAuthIndex.get(observation.id)
    if (!existing || observationTime(observation) > observationTime(existing)) {
      byAuthIndex.set(observation.id, observation)
    }
  }
  return { items: [...byAuthIndex.values()] }
}

function refreshErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message) return error.message
  return "Refresh failed"
}

export function useLiveCapacity(provider: string) {
  const queryClient = useQueryClient()
  const [taskStates, setTaskStates] = useState<Record<string, LiveCapacityTaskState>>({})

  const identitiesQuery = useQuery({
    queryKey: ["quota", "auth-file-identities"],
    queryFn: fetchAllAuthFileIdentities,
    staleTime: 60_000,
  })

  const identities = useMemo(() => identitiesQuery.data ?? [], [identitiesQuery.data])
  const visibleIdentities = useMemo(
    () => identities.filter((identity) => matchesProvider(identity, provider)),
    [identities, provider],
  )
  const visibleAuthIndexes = useMemo(
    () => visibleIdentities.map((identity) => identity.identity).filter(Boolean),
    [visibleIdentities],
  )

  const observationsQueryKey = quotaObservationsQueryKey(provider, visibleIdentities)
  const observationsQuery = useQuery({
    queryKey: observationsQueryKey,
    queryFn: async () => {
      const snapshot = await fetchQuotaObservations(visibleAuthIndexes)
      return mergeQuotaObservations(
        queryClient.getQueryData<QuotaObservationsResponse>(observationsQueryKey),
        snapshot,
      )
    },
    enabled: visibleAuthIndexes.length > 0,
    staleTime: 30_000,
  })

  const refreshMutation = useMutation({
    mutationFn: requestQuotaRefresh,
    onMutate: (authIndexes) => {
      setTaskStates((current) => {
        const next = { ...current }
        for (const authIndex of authIndexes) {
          next[authIndex] = { status: "starting" }
        }
        return next
      })
    },
    onSuccess: ({ responses, failedAuthIndexes }) => {
      const taskIds = responses.flatMap((response) => response.tasks)
      const rejected = responses.flatMap((response) => response.rejected ?? [])
      setTaskStates((current) => {
        const next = { ...current }
        for (const task of taskIds) {
          next[task.authIndex] = { status: "queued", taskId: task.taskId }
        }
        for (const item of rejected) {
          next[item.authIndex] = { status: "failed", error: item.error }
        }
        for (const item of failedAuthIndexes) {
          next[item.authIndex] = { status: "failed", error: item.error }
        }
        return next
      })
    },
  })

  const disabledAuthIndexes = useMemo(
    () => new Set(identities.filter((identity) => identity.disabled === true).map((identity) => identity.identity)),
    [identities],
  )

  const refresh = useCallback((target?: string | string[]) => {
    const requestedAuthIndexes = (target === undefined
      ? visibleAuthIndexes
      : Array.isArray(target)
        ? target
        : [target]
    ).filter((authIndex) => !disabledAuthIndexes.has(authIndex))
    const authIndexes = selectRefreshAuthIndexes({
      requestedAuthIndexes,
      taskStates,
      limit: typeof target === "string" ? 1 : QUOTA_REFRESH_LIMIT,
    })
    if (authIndexes.length === 0) return
    refreshMutation.mutate(authIndexes)
  }, [refreshMutation, taskStates, visibleAuthIndexes, disabledAuthIndexes])

  useQuery({
    queryKey: [
      "quota",
      "refresh-tasks",
      activeTaskFingerprint(taskStates),
    ],
    queryFn: async () => {
      const result = await resolveRefreshTaskUpdates(taskStates, fetchRefreshTask)
      setTaskStates((current) => {
        const next = { ...current }
        for (const [authIndex, update] of Object.entries(result.updates)) {
          next[authIndex] = update
        }
        return next
      })
      if (result.completedObservations.length > 0) {
        queryClient.setQueryData<QuotaObservationsResponse>(
          observationsQueryKey,
          (current) => mergeQuotaObservations(current, { items: result.completedObservations }),
        )
        void queryClient.invalidateQueries({ queryKey: ["quota", "observations"] })
      }
      return result
    },
    enabled: hasActiveTasks(taskStates),
    refetchInterval: 1_500,
  })

  return {
    identities: visibleIdentities,
    observations: observationsQuery.data,
    taskStates,
    refresh,
    refreshLimit: QUOTA_REFRESH_LIMIT,
    isLoading: identitiesQuery.isLoading || observationsQuery.isLoading,
    isRefreshing: refreshMutation.isPending || Object.values(taskStates).some(isAnyRefreshState),
    error: identitiesQuery.error ?? observationsQuery.error,
  }
}
