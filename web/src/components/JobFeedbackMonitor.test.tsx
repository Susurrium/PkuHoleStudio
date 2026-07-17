import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it } from 'vitest'
import type { Job } from '../lib/types'
import { useUIStore } from '../store/ui'
import { FeedbackProvider } from './Feedback'
import { JobFeedbackMonitor } from './JobFeedbackMonitor'

const base: Job = { id: 'save-1', type: 'sync_pids', status: 'running', scope: { pids: [101, 102] }, completed_items: 0, failed_items: 0, total_items: 2, attempts: 1, created_at: '2026-07-16T00:00:00Z', updated_at: '2026-07-16T00:00:01Z' }

afterEach(() => {
  window.sessionStorage.clear()
  useUIStore.getState().setLayoutPreset('studio')
})

describe('JobFeedbackMonitor', () => {
  it('announces a new terminal transition once and carries saved PIDs into local organization', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    client.setQueryData(['jobs'], [base])
    render(<QueryClientProvider client={client}><MemoryRouter><FeedbackProvider><JobFeedbackMonitor /></FeedbackProvider></MemoryRouter></QueryClientProvider>)
    await act(async () => undefined)
    act(() => client.setQueryData(['jobs'], [{ ...base, status: 'completed', completed_items: 2, updated_at: '2026-07-16T00:00:02Z' }]))
    expect(await screen.findByText('同步指定 PID 已完成')).toBeInTheDocument()
    await userEvent.setup().click(screen.getByRole('link', { name: '整理这些树洞' }))
    expect(JSON.parse(window.sessionStorage.getItem('pkustudio:local-selection') || '{}').pids).toEqual([101, 102])
    act(() => client.setQueryData(['jobs'], [{ ...base, status: 'completed', completed_items: 2, updated_at: '2026-07-16T00:00:03Z' }]))
    await waitFor(() => expect(screen.queryAllByText('同步指定 PID 已完成')).toHaveLength(0))
  })

  it('does not announce terminal history from the initial query snapshot', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    client.setQueryData(['jobs'], [{ ...base, status: 'failed', error: '历史错误' }])
    render(<QueryClientProvider client={client}><MemoryRouter><FeedbackProvider><JobFeedbackMonitor /></FeedbackProvider></MemoryRouter></QueryClientProvider>)
    await act(async () => undefined)
    expect(screen.queryByText('同步指定 PID失败')).not.toBeInTheDocument()
  })
})
