import { useQuery, type QueryClient } from '@tanstack/react-query'
import { api } from '../../lib/api'

export function usePostUpdatesAvailable() {
	const capabilities = useQuery({ queryKey: ['capabilities'], queryFn: api.capabilities })
	return capabilities.data?.post_updates === true
}

export function usePostUpdatesState() {
	const available = usePostUpdatesAvailable()
	return useQuery({ queryKey: ['post-updates'], queryFn: api.postUpdates, enabled: available, retry: false, refetchInterval: 10_000 })
}

export function invalidatePostContent(client: QueryClient, pid?: string | number) {
	const keys = [pid === undefined ? ['post'] : ['post', String(pid)], ['posts'], ['health'], ['projects'], ['post-references'], ['reference-graph'], ['online-home-latest']]
	return Promise.all(keys.map((queryKey) => client.invalidateQueries({ queryKey })))
}
