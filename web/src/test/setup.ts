import '@testing-library/jest-dom/vitest'

// jsdom exposes scrollTo but reports it as unimplemented; list restoration is
// covered through its observable storage/navigation behavior in component tests.
Object.defineProperty(window, 'scrollTo', { configurable: true, writable: true, value: () => undefined })
