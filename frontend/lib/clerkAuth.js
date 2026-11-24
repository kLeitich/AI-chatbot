import api from './api'

/**
 * Extracts the Clerk Frontend API domain from the publishable key.
 * e.g. pk_test_Y2xlcmsuZXhhbXBsZS5jb20k -> clerk.example.com
 */
export function getClerkFrontendApi(publishableKey) {
  if (!publishableKey) return null
  try {
    const parts = publishableKey.split('_')
    if (parts.length >= 3) {
      const b64 = parts[2].replace(/-/g, '+').replace(/_/g, '/')
      const padded = b64.padEnd(b64.length + ((4 - (b64.length % 4)) % 4), '=')
      const decoded = typeof window !== 'undefined' ? atob(padded) : ''
      return decoded.endsWith('$') ? decoded.slice(0, -1) : decoded
    }
  } catch (e) {
    // ignore
  }
  return null
}

/**
 * Loads the Clerk JS SDK and UI components dynamically.
 */
export async function loadClerkSDK(clerkPublishableKey = '', onProgress = null) {
  if (typeof window === 'undefined') return null

  const clerkKey =
    clerkPublishableKey ||
    process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY ||
    window.CLERK_PUBLISHABLE_KEY

  if (!clerkKey) {
    throw new Error(
      'Clerk SSO is not configured yet. Please set NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY in your environment variables.'
    )
  }

  if (window.Clerk?.loaded) {
    return window.Clerk
  }

  const frontendApi = getClerkFrontendApi(clerkKey)

  // 1. Load Clerk UI bundle if not present to enable UI components (openSignIn, openSignUp)
  if (!window.__internal_ClerkUICtor) {
    await new Promise((resolve) => {
      const uiScript = document.createElement('script')
      uiScript.src = frontendApi
        ? `https://${frontendApi}/npm/@clerk/ui@1/dist/ui.browser.js`
        : 'https://cdn.jsdelivr.net/npm/@clerk/ui@latest/dist/ui.browser.js'
      uiScript.async = true
      uiScript.crossOrigin = 'anonymous'
      uiScript.onload = () => resolve()
      uiScript.onerror = () => {
        // Continue even if UI bundle script fails - we have direct OAuth fallback
        resolve()
      }
      document.head.appendChild(uiScript)
    })
  }

  // 2. Load Clerk JS SDK core
  if (!window.Clerk) {
    if (onProgress) onProgress('Loading Google authentication...')
    await new Promise((resolve, reject) => {
      const script = document.createElement('script')
      script.src = frontendApi
        ? `https://${frontendApi}/npm/@clerk/clerk-js@5/dist/clerk.browser.js`
        : 'https://cdn.jsdelivr.net/npm/@clerk/clerk-js@5/dist/clerk.browser.js'
      script.async = true
      script.crossOrigin = 'anonymous'
      script.setAttribute('data-clerk-publishable-key', clerkKey)
      script.onload = () => resolve()
      script.onerror = () => reject(new Error('Failed to load Clerk authentication library'))
      document.head.appendChild(script)
    })
  }

  if (!window.Clerk) {
    throw new Error('Clerk SDK could not be initialized')
  }

  if (!window.Clerk.loaded) {
    const loadOptions = { publishableKey: clerkKey }
    if (window.__internal_ClerkUICtor) {
      loadOptions.ui = { ClerkUI: window.__internal_ClerkUICtor }
    }
    await window.Clerk.load(loadOptions)
  }

  return window.Clerk
}

/**
 * Initiates Clerk authentication with Google and synchronizes the session with Booka.
 * Supports both modal presentation and fallback direct Google OAuth redirect.
 */
export async function initiateClerkGoogleAuth({
  mode = 'signIn', // 'signIn' or 'signUp'
  tenant = '',
  companyName = '',
  clerkPublishableKey = '',
  onProgress = null,
  onSuccess = null,
  onError = null,
} = {}) {
  const clerkKey =
    clerkPublishableKey ||
    process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY ||
    (typeof window !== 'undefined' ? window.CLERK_PUBLISHABLE_KEY : null)

  if (!clerkKey) {
    const errorMsg =
      'Clerk SSO is not configured yet. Please set NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY in your environment variables.'
    if (onError) onError(errorMsg)
    throw new Error(errorMsg)
  }

  let clerk = null
  try {
    clerk = await loadClerkSDK(clerkKey, onProgress)
  } catch (err) {
    if (onError) onError(err.message)
    throw err
  }

  // Try opening UI modal first if UI components are present
  let modalOpened = false
  try {
    if (window.__internal_ClerkUICtor || clerk.ui) {
      if (mode === 'signUp' && typeof clerk.openSignUp === 'function') {
        clerk.openSignUp({ afterSignUpUrl: window.location.href })
        modalOpened = true
      } else if (typeof clerk.openSignIn === 'function') {
        clerk.openSignIn({ afterSignInUrl: window.location.href })
        modalOpened = true
      }
    }
  } catch (uiErr) {
    console.warn('Clerk UI modal unavailable, falling back to direct Google redirect', uiErr)
    modalOpened = false
  }

  // If modal cannot be opened (e.g. headless or missing UI bundle),
  // directly trigger Google OAuth redirect via Clerk!
  if (!modalOpened) {
    if (onProgress) onProgress('Connecting with Google...')

    if (typeof window !== 'undefined') {
      try {
        sessionStorage.setItem(
          'booka_clerk_pending_sync',
          JSON.stringify({ tenant, companyName, mode, ts: Date.now() })
        )
      } catch (e) {
        // ignore
      }
    }

    const currentUrl = window.location.href

    if (mode === 'signUp' && clerk.client?.signUp?.authenticateWithRedirect) {
      await clerk.client.signUp.authenticateWithRedirect({
        strategy: 'oauth_google',
        redirectUrl: currentUrl,
        redirectUrlComplete: currentUrl,
      })
      return
    }

    if (clerk.client?.signIn?.authenticateWithRedirect) {
      await clerk.client.signIn.authenticateWithRedirect({
        strategy: 'oauth_google',
        redirectUrl: currentUrl,
        redirectUrlComplete: currentUrl,
      })
      return
    }

    // Direct fallback if available on clerk root
    if (typeof clerk.authenticateWithRedirect === 'function') {
      await clerk.authenticateWithRedirect({
        strategy: 'oauth_google',
        redirectUrl: currentUrl,
        redirectUrlComplete: currentUrl,
      })
      return
    }
  }

  // Poll for active session after user authenticates in modal
  return new Promise((resolve, reject) => {
    let attempts = 0
    const checkInterval = setInterval(async () => {
      attempts++
      if (window.Clerk?.session) {
        clearInterval(checkInterval)
        try {
          if (onProgress) onProgress('Verifying Google credentials with clinic...')
          const token = await window.Clerk.session.getToken()
          if (!token) {
            throw new Error('No authentication session token returned by Clerk')
          }

          const syncEndpoint = tenant ? `/t/${tenant}/auth/clerk/sync` : '/auth/clerk/sync'
          const res = await api.post(syncEndpoint, {
            token,
            company_name: companyName,
          })

          const data = res.data || {}
          const { token: bookaToken, user } = data

          if (bookaToken) {
            localStorage.setItem('token', bookaToken)
            if (user?.role) localStorage.setItem('role', user.role)
            if (user?.tenant_id) localStorage.setItem('tenant_id', String(user.tenant_id))
          }

          if (onSuccess) onSuccess(data)
          resolve(data)
        } catch (err) {
          const errMsg =
            err?.response?.data?.message || err?.response?.data?.error || err?.message || 'Authentication failed'
          if (onError) onError(errMsg)
          reject(err)
        }
      }

      if (attempts > 180) {
        clearInterval(checkInterval)
      }
    }, 1000)
  })
}

/**
 * Checks if the user already has an active Clerk session (e.g. after returning from Google OAuth redirect)
 * and exchanges it for a native Booka session.
 */
export async function checkAndSyncClerkSession({
  tenant = '',
  clerkPublishableKey = '',
  onProgress = null,
  onSuccess = null,
  onError = null,
} = {}) {
  if (typeof window === 'undefined') return null

  const clerkKey =
    clerkPublishableKey ||
    process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY ||
    window.CLERK_PUBLISHABLE_KEY

  if (!clerkKey) return null

  try {
    const clerk = await loadClerkSDK(clerkKey)
    if (!clerk?.session) return null

    if (onProgress) onProgress('Completing Google authentication...')
    const token = await clerk.session.getToken()
    if (!token) return null

    let companyName = ''
    let targetTenant = tenant
    try {
      const stored = sessionStorage.getItem('booka_clerk_pending_sync')
      if (stored) {
        const parsed = JSON.parse(stored)
        if (parsed.companyName) companyName = parsed.companyName
        if (parsed.tenant && !targetTenant) targetTenant = parsed.tenant
        sessionStorage.removeItem('booka_clerk_pending_sync')
      }
    } catch (e) {
      // ignore
    }

    const syncEndpoint = targetTenant ? `/t/${targetTenant}/auth/clerk/sync` : '/auth/clerk/sync'
    const res = await api.post(syncEndpoint, {
      token,
      company_name: companyName,
    })

    const data = res.data || {}
    const { token: bookaToken, user } = data

    if (bookaToken) {
      localStorage.setItem('token', bookaToken)
      if (user?.role) localStorage.setItem('role', user.role)
      if (user?.tenant_id) localStorage.setItem('tenant_id', String(user.tenant_id))
    }

    if (onSuccess) onSuccess(data)
    return data
  } catch (err) {
    if (onError) onError(err?.response?.data?.message || err?.message || 'Failed to sync Google session')
    return null
  }
}
