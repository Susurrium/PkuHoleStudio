import { describe, expect, it } from 'vitest'
import { navigationTargetIsActive, workspaceForLocation } from './WorkspaceSwitcher'

describe('workspace routing', () => {
  it('maps online and library pages without assigning global utility pages', () => {
    expect(workspaceForLocation('/online', '')).toBe('online')
    expect(workspaceForLocation('/posts', '?source=live')).toBe('online')
    expect(workspaceForLocation('/posts/123', '?source=live')).toBe('online')
    expect(workspaceForLocation('/notifications', '')).toBe('online')
    expect(workspaceForLocation('/campus', '')).toBe('online')
    expect(workspaceForLocation('/', '')).toBe('library')
    expect(workspaceForLocation('/posts', '')).toBe('library')
    expect(workspaceForLocation('/ai', '')).toBe('library')
    expect(workspaceForLocation('/projects', '?project=1')).toBe('library')
    expect(workspaceForLocation('/removed/123', '')).toBe('library')
    expect(workspaceForLocation('/tasks', '')).toBeNull()
    expect(workspaceForLocation('/settings', '')).toBeNull()
  })

  it('distinguishes online list intents that share the same pathname', () => {
    expect(navigationTargetIsActive('/posts', '?source=live', '', '/posts?source=live')).toBe(true)
    expect(navigationTargetIsActive('/posts', '?source=live&followed=true', '', '/posts?source=live')).toBe(false)
    expect(navigationTargetIsActive('/posts', '?source=live&followed=true', '', '/posts?source=live&followed=true')).toBe(true)
    expect(navigationTargetIsActive('/posts', '?source=live&compose=true', '', '/posts?source=live&compose=true')).toBe(true)
    expect(navigationTargetIsActive('/posts', '', '', '/posts?source=live')).toBe(false)
    expect(navigationTargetIsActive('/online', '', '#hot', '/online#hot')).toBe(true)
    expect(navigationTargetIsActive('/online', '', '#hot', '/online', true)).toBe(false)
  })
})
