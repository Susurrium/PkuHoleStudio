import { afterEach, describe, expect, it, vi } from 'vitest'
import { preferredScrollBehavior } from './motion'

describe('preferredScrollBehavior', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('disables smooth scrolling when reduced motion is requested', () => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true })) as unknown as typeof window.matchMedia)
    expect(preferredScrollBehavior()).toBe('auto')
  })

  it('keeps smooth scrolling for the default motion preference', () => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false })) as unknown as typeof window.matchMedia)
    expect(preferredScrollBehavior()).toBe('smooth')
  })
})
