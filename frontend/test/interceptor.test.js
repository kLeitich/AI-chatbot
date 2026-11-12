import { describe, it, expect, vi, beforeEach } from 'vitest'
import api, { nav } from '../lib/api'

describe('Auth Interceptors (401 vs 403)', () => {
  let responseHandler
  let redirectSpy
  let getPathnameSpy

  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()

    // Grab the response rejected handler from the axios instance
    responseHandler = api.interceptors.response.handlers[0].rejected

    // Spy on navigation helpers
    redirectSpy = vi.spyOn(nav, 'redirect').mockImplementation(() => {})
    getPathnameSpy = vi.spyOn(nav, 'getPathname').mockReturnValue('/t/test-clinic/admin/dashboard')

    // Clean up any lingering toast elements
    const existing = document.getElementById('toast-permission-denied')
    if (existing) existing.remove()
  })

  it('clears token and redirects to tenant login with notice on 401 TOKEN_EXPIRED', async () => {
    localStorage.setItem('token', 'expired-token')
    localStorage.setItem('tenant_id', '123')

    const err = {
      config: { url: '/t/test-clinic/appointments' },
      response: {
        status: 401,
        data: {
          code: 'TOKEN_EXPIRED',
          message: 'Token has expired',
        },
      },
    }

    await expect(responseHandler(err)).rejects.toEqual(err)

    // Token must be cleared
    expect(localStorage.getItem('token')).toBeNull()
    expect(localStorage.getItem('tenant_id')).toBeNull()

    // Redirected to tenant login with notice
    expect(redirectSpy).toHaveBeenCalledWith('/t/test-clinic/login?notice=session_ended')
  })

  it('redirects to platform login with notice on 401 from platform path', async () => {
    getPathnameSpy.mockReturnValue('/platform/dashboard')
    localStorage.setItem('token', 'expired-platform-token')

    const err = {
      config: { url: '/platform/tenants' },
      response: {
        status: 401,
        data: {
          code: 'TOKEN_EXPIRED',
          message: 'Token expired',
        },
      },
    }

    await expect(responseHandler(err)).rejects.toEqual(err)

    expect(localStorage.getItem('token')).toBeNull()
    expect(redirectSpy).toHaveBeenCalledWith('/platform/login?notice=session_ended')
  })

  it('avoids redirect loop if 401 happens on login page or during login request', async () => {
    getPathnameSpy.mockReturnValue('/t/test-clinic/login')

    const err = {
      config: { url: '/t/test-clinic/login' },
      response: {
        status: 401,
        data: {
          code: 'INVALID_CREDENTIALS',
          message: 'Invalid credentials',
        },
      },
    }

    await expect(responseHandler(err)).rejects.toEqual(err)

    // Should NOT have triggered redirect (no redirect loop)
    expect(redirectSpy).not.toHaveBeenCalled()
  })

  it('shows toast and does NOT log out or redirect on 403 PERMISSION_DENIED', async () => {
    localStorage.setItem('token', 'valid-active-token')

    const err = {
      config: { url: '/t/test-clinic/admin/settings' },
      response: {
        status: 403,
        data: {
          code: 'PERMISSION_DENIED',
          message: 'You lack settings.manage permission',
        },
      },
    }

    await expect(responseHandler(err)).rejects.toEqual(err)

    // Token must NOT be cleared
    expect(localStorage.getItem('token')).toBe('valid-active-token')

    // Must NOT redirect
    expect(redirectSpy).not.toHaveBeenCalled()

    // Toast element should be present in document with message
    const toast = document.getElementById('toast-permission-denied')
    expect(toast).not.toBeNull()
    expect(toast.textContent).toContain('You lack settings.manage permission')
  })
})
