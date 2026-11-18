import { describe, it, expect, vi, beforeEach } from 'vitest'
import { middleware, clearOriginCache } from '../middleware'

describe('Next.js Middleware Security Headers', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    clearOriginCache()
  })

  it('sets frame-ancestors none and X-Frame-Options DENY on admin and standard routes', async () => {
    const mockRequest = {
      url: 'https://example.com/t/demo/admin/dashboard',
      nextUrl: { pathname: '/t/demo/admin/dashboard' },
    }

    const response = await middleware(mockRequest)
    expect(response.headers.get('Content-Security-Policy')).toBe("frame-ancestors 'none'")
    expect(response.headers.get('X-Frame-Options')).toBe('DENY')
    expect(response.headers.get('X-Content-Type-Options')).toBe('nosniff')
    expect(response.headers.get('Referrer-Policy')).toBe('strict-origin-when-cross-origin')
  })

  it('sets per-tenant frame-ancestors on /embed/:tenant and omits X-Frame-Options when origins exist', async () => {
    const originalFetch = global.fetch
    global.fetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        allowed_embed_origins: ['https://partner.com', 'https://*.hospital.org'],
      }),
    })

    const mockRequest = {
      url: 'https://example.com/embed/demo-clinic',
      nextUrl: { pathname: '/embed/demo-clinic' },
    }

    const response = await middleware(mockRequest)
    expect(response.headers.get('Content-Security-Policy')).toBe(
      'frame-ancestors https://partner.com https://*.hospital.org'
    )
    expect(response.headers.get('X-Frame-Options')).toBeNull()

    global.fetch = originalFetch
  })

  it('fails closed to frame-ancestors none and DENY when allowed_embed_origins is empty', async () => {
    const originalFetch = global.fetch
    global.fetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        allowed_embed_origins: [],
      }),
    })

    const mockRequest = {
      url: 'https://example.com/embed/empty-origins-clinic',
      nextUrl: { pathname: '/embed/empty-origins-clinic' },
    }

    const response = await middleware(mockRequest)
    expect(response.headers.get('Content-Security-Policy')).toBe("frame-ancestors 'none'")
    expect(response.headers.get('X-Frame-Options')).toBe('DENY')

    global.fetch = originalFetch
  })

  it('fails closed to frame-ancestors none and DENY when public-config fetch fails', async () => {
    const originalFetch = global.fetch
    global.fetch = vi.fn().mockRejectedValueOnce(new Error('Network error'))

    const mockRequest = {
      url: 'https://example.com/embed/failing-clinic',
      nextUrl: { pathname: '/embed/failing-clinic' },
    }

    const response = await middleware(mockRequest)
    expect(response.headers.get('Content-Security-Policy')).toBe("frame-ancestors 'none'")
    expect(response.headers.get('X-Frame-Options')).toBe('DENY')

    global.fetch = originalFetch
  })
})
