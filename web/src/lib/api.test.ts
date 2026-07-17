import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'

afterEach(() => vi.restoreAllMocks())

function response(data: unknown, status = 200) {
	const body = status >= 400 ? { error: data } : { data }
	return Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }))
}

describe('media upload retry cache', () => {
	it('reuses successful media ids while retrying only failed files', async () => {
		const attempts = new Map<string, number>()
		vi.stubGlobal('fetch', vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
			const file = (init?.body as FormData).get('file') as File
			const count = (attempts.get(file.name) ?? 0) + 1
			attempts.set(file.name, count)
			if (file.name === 'second.png' && count === 1) return response({ code: 'upload_failed', message: 'temporary failure' }, 502)
			return response({ id: file.name === 'first.png' ? '101' : '102', filename: file.name, size: file.size }, 201)
		}))
		const first = new File(['first'], 'first.png', { type: 'image/png' })
		const second = new File(['second'], 'second.png', { type: 'image/png' })

		await expect(api.uploadMediaIDs([first, second])).rejects.toThrow('temporary failure')
		await expect(api.uploadMediaIDs([first, second])).resolves.toEqual(['101', '102'])
		expect(attempts.get('first.png')).toBe(1)
		expect(attempts.get('second.png')).toBe(2)
	})
})
