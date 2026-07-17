import { Navigate, useSearchParams } from 'react-router-dom'

export function SearchPage() {
  const [params] = useSearchParams()
  const next = new URLSearchParams(params)
  if (!next.has('focus') && !next.has('q')) next.set('focus', 'search')
  return <Navigate to={`/posts?${next}`} replace />
}
