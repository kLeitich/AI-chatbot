import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  getClerkFrontendApi,
  loadClerkSDK,
  initiateClerkGoogleAuth,
  checkAndSyncClerkSession,
  signOutClerkSession,
} from '../lib/clerkAuth'
import api from '../lib/api'

const mockLoad = vi.fn()
const mockOpenSignIn = vi.fn()
const mockOpenSignUp = vi.fn()
const mockAddListener = vi.fn()
const mockSignOut = vi.fn()
const mockGetToken = vi.fn()
const mockSignInRedirect = vi.fn()
const mockSignUpRedirect = vi.fn()
const mockHandleRedirectCallback = vi.fn()

let mockClerkInstance = null

vi.mock('@clerk/clerk-js', () => ({
  Clerk: vi.fn(function (key) {
    mockClerkInstance = {
      publishableKey: key,
      loaded: false,
      session: null,
      environment: {},
      frontendApi: 'example.clerk.accounts.dev',
      load: async () => {
        await mockLoad(key)
        mockClerkInstance.loaded = true
      },
      openSignIn: mockOpenSignIn,
      openSignUp: mockOpenSignUp,
      addListener: mockAddListener,
      signOut: mockSignOut,
      handleRedirectCallback: mockHandleRedirectCallback,
      client: {
        signIn: { authenticateWithRedirect: mockSignInRedirect },
        signUp: { authenticateWithRedirect: mockSignUpRedirect },
      },
    }
    return mockClerkInstance
  }),
}))

vi.mock('../lib/api', () => ({
  default: {
    post: vi.fn(),
  },
}))

describe('clerkAuth bundled helper', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    delete process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY
    delete window.Clerk
    window.history.replaceState({}, '', '/login')
  })

  it('decodes Frontend API domain from publishable key', () => {
    expect(getClerkFrontendApi('pk_test_ZXhhbXBsZS5jbGVyay5hY2NvdW50cy5kZXYk')).toBe('example.clerk.accounts.dev')
    expect(getClerkFrontendApi('  "pk_test_ZXhhbXBsZS5jbGVyay5hY2NvdW50cy5kZXYk"  ')).toBe('example.clerk.accounts.dev')
  })

  it('throws clear error when publishable key is missing', async () => {
    await expect(loadClerkSDK('')).rejects.toThrow(/NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY/i)
  })

  it('initializes bundled Clerk SDK without external script tags', async () => {
    const appendSpy = vi.spyOn(document.head, 'appendChild')
    process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY = 'pk_test_dGVzdC1kb21haW4uY2xlcmsk'

    const clerk = await loadClerkSDK()
    expect(clerk).toBeDefined()
    expect(clerk.loaded).toBe(true)
    expect(mockLoad).toHaveBeenCalledWith('pk_test_dGVzdC1kb21haW4uY2xlcmsk')
    expect(appendSpy).not.toHaveBeenCalled()
    appendSpy.mockRestore()
  })

  it('redirects directly to Google OAuth account chooser without intermediate Clerk modal', async () => {
    mockLoad.mockResolvedValue(undefined)
    const res = await initiateClerkGoogleAuth({
      mode: 'signIn',
      clerkPublishableKey: 'pk_test_ZGlyZWN0LWdvb2dsZS1yZWRpcmVjdCQ=',
    })

    expect(mockOpenSignIn).not.toHaveBeenCalled()
    expect(mockSignInRedirect).toHaveBeenCalledWith({
      strategy: 'oauth_google',
      redirectUrl: 'http://localhost:3000/login?sso_callback=1',
      redirectUrlComplete: 'http://localhost:3000/login?sso_complete=1',
    })
    expect(res).toEqual({ redirected: true })
  })

  it('syncs existing active Clerk session directly without redirect', async () => {
    mockGetToken.mockResolvedValue('clerk_jwt_123')
    mockLoad.mockImplementation(async () => {
      mockClerkInstance.session = { getToken: mockGetToken }
    })
    api.post.mockResolvedValueOnce({
      data: {
        token: 'booka_jwt_abc',
        user: { role: 'tenant_admin', tenant_id: 7 },
        tenant: { slug: 'acme-clinic' },
      },
    })

    const onSuccess = vi.fn()
    const res = await initiateClerkGoogleAuth({
      clerkPublishableKey: 'pk_test_YWN0aXZlLXNlc3Npb24k',
      companyName: 'Acme Clinic',
      onSuccess,
    })

    expect(mockSignInRedirect).not.toHaveBeenCalled()
    expect(api.post).toHaveBeenCalledWith('/auth/clerk/sync', {
      token: 'clerk_jwt_123',
      company_name: 'Acme Clinic',
    })
    expect(localStorage.getItem('token')).toBe('booka_jwt_abc')
    expect(onSuccess).toHaveBeenCalledWith(res)
  })

  it('handles returning Google OAuth callback via handleRedirectCallback and ignores stale session_ended', async () => {
    mockLoad.mockResolvedValue(undefined)
    mockGetToken.mockResolvedValue('clerk_jwt_return')
    mockHandleRedirectCallback.mockImplementation(async () => {
      mockClerkInstance.session = { getToken: mockGetToken }
    })

    window.history.replaceState({}, '', '/login?notice=session_ended')
    const skipped = await checkAndSyncClerkSession({
      clerkPublishableKey: 'pk_test_c2Vzc2lvbi1lbmRlZCQ=',
    })
    expect(skipped).toBeNull()

    window.history.replaceState({}, '', '/login?sso_callback=1')
    api.post.mockResolvedValueOnce({
      data: {
        token: 'booka_jwt_return',
        user: { role: 'staff', tenant_id: 9 },
        tenant: { slug: 'beta-clinic' },
      },
    })

    const synced = await checkAndSyncClerkSession({
      clerkPublishableKey: 'pk_test_c2Vzc2lvbi1lbmRlZCQ=',
    })
    expect(mockHandleRedirectCallback).toHaveBeenCalled()
    expect(synced?.token).toBe('booka_jwt_return')
    expect(localStorage.getItem('token')).toBe('booka_jwt_return')

    await signOutClerkSession()
    expect(sessionStorage.getItem('booka_clerk_logged_out')).toBe('1')
  })
})
