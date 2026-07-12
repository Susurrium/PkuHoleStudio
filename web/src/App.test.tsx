import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import App from './App'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

function renderApp(path = '/') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><App /></MemoryRouter></QueryClientProvider>)
}

function json(data: unknown, status = 200) {
  return Promise.resolve(new Response(JSON.stringify({ data }), { status, headers: { 'Content-Type': 'application/json' } }))
}

describe('PkuHoleStudio Web', () => {
  it('shows the empty archive guidance on the dashboard', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const path = String(input)
      if (path.endsWith('/health')) return json({ status: 'ok', posts: 0, comments: 0 })
      if (path.endsWith('/capabilities')) return json({ api_version: 'v1', schema_version: 3, fts5: true, archive_import: true, jobs: true, ai: false, live_search: false })
      if (path.includes('/jobs')) return json([])
      throw new Error(`unexpected request ${path}`)
    }))
    renderApp('/')
    expect(await screen.findByText('资料库还是空的')).toBeInTheDocument()
    expect(screen.getByText('FTS5')).toBeInTheDocument()
  })

  it('keeps a search in the URL-backed page and renders a result', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const path = String(input)
		if (path.includes('/search?q=alpha')) return json({ items: [{ pid: 123456, text: 'alpha result', reply: 2 }], has_more: false })
		if (path.includes('/search/history')) return json([])
      throw new Error(`unexpected request ${path}`)
    }))
    const user = userEvent.setup()
    renderApp('/search')
    await user.type(screen.getByPlaceholderText('课程名、教师、关键词或 #PID'), 'alpha')
    await user.click(screen.getByRole('button', { name: '搜索资料库' }))
    expect(await screen.findByText('alpha result')).toBeInTheDocument()
    expect(screen.getByText('#123456')).toBeInTheDocument()
  })

  it('uploads an archive and displays its preflight report', async () => {
    vi.stubGlobal('fetch', vi.fn(() => json({
      job: { id: 'job-1', type: 'import_archive', status: 'queued', completed_items: 0, failed_items: 0, total_items: 1, attempts: 0, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
      preflight: { format: 'legacy-v1', status: 'completed', hash: 'abc', run_id: 'legacy-abc', counts: { items: 1, comments: 0 }, issues: [] },
    })))
    const user = userEvent.setup()
    const { container } = renderApp('/imports')
    const input = container.querySelector('input[type=file]') as HTMLInputElement
    await user.upload(input, new File(['{"holes":[]}'], 'archive.json', { type: 'application/json' }))
    await user.click(screen.getByRole('button', { name: '预检并开始导入' }))
    await waitFor(() => expect(screen.getByText('预检完成 · legacy-v1')).toBeInTheDocument())
    expect(screen.getByText('job-1')).toBeInTheDocument()
  })
})
