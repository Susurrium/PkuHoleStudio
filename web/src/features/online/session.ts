import { useQuery, type QueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { AuthStatus } from '../../lib/types'

export const onlineSessionQueryKey = ['online-session'] as const

export function useOnlineSession(enabled = true) {
  return useQuery({
    queryKey: onlineSessionQueryKey,
    queryFn: api.probeSession,
    enabled,
    retry: false,
    staleTime: 30_000,
    gcTime: 5 * 60_000,
    refetchOnWindowFocus: false,
  })
}

export function setOnlineSession(client: QueryClient, value: AuthStatus) {
  client.setQueryData(onlineSessionQueryKey, value)
}

export function invalidateOnlineSession(client: QueryClient) {
  return client.invalidateQueries({ queryKey: onlineSessionQueryKey })
}

export function useSlowLoading(loading: boolean, delay = 2_500) {
  const [slow, setSlow] = useState(false)
  useEffect(() => {
    if (!loading) {
      setSlow(false)
      return
    }
    const timer = window.setTimeout(() => setSlow(true), delay)
    return () => window.clearTimeout(timer)
  }, [delay, loading])
  return slow
}
