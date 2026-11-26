import { Clerk } from '@clerk/clerk-js'
import api from './api'

const clerkInstances = {}

function normalizeClerkKey(rawKey) {
  if (!rawKey || typeof rawKey !== 'string') return ''
  return rawKey.trim().replace(/^['"]|['"]$/g, '').trim()
}

function formatClerkError(err, fallbackMsg) {
  const apiMsg = err?.errors?.[0]?.longMessage || err?.errors?.[0]?.message
  const httpMsg = err?.response?.data?.message || err?.response?.data?.error
  return apiMsg || httpMsg || err?.message || fallbackMsg
}

/**
 * Extracts the Clerk Frontend API domain from the publishable key.
 * e.g. pk_test_Y2xlcmsuZXhhbXBsZS5jb20k -> clerk.example.com
 */
export function getClerkFrontendApi(publishableKey) {
  const clean = normalizeClerkKey(publishableKey)
  if (!clean) return null
  try {
    const parts = clean.split('_')
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
 * Loads and initializes the bundled Clerk JS SDK with built-in UI components.
 */
export async function loadClerkSDK(clerkPublishableKey = '', onProgress = null) {
  if (typeof window === 'undefined') return null

  const clerkKey = normalizeClerkKey(
    clerkPublishableKey ||
      process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY ||
      window.CLERK_PUBLISHABLE_KEY
  )

  if (!clerkKey) {
    throw new Error(
      'Clerk SSO is not configured yet. Please set NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY in your environment variables.'
    )
  }

  let clerk = clerkInstances[clerkKey]
  if (clerk?.loaded) {
    return clerk
  }

  if (onProgress) onProgress('Initializing Google authentication...')

  try {
    if (!clerk) {
      clerk = new Clerk(clerkKey)
      clerkInstances[clerkKey] = clerk
      window.Clerk = clerk
    }

    if (!clerk.loaded) {
      await clerk.load()
    }

    return clerk
  } catch (err) {
    delete clerkInstances[clerkKey]
    const detailed = formatClerkError(
      err,
      'Failed to initialize Clerk authentication. Verify NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY and domain settings.'
    )
    throw new Error(detailed)
  }
}

async function getClerkTokenWithRetry(clerk) {
  for (let i = 0; i < 4; i++) {
    if (clerk?.session) {
      const tok = await clerk.session.getToken()
      if (tok) return tok
    }
    await new Promise((r) => setTimeout(r, 250))
  }
  return null
}

async function exchangeClerkSessionWithBooka(
  clerk,
  { tenant = '', companyName = '', onProgress = null, onSuccess = null, onError = null }
) {
  try {
    if (onProgress) onProgress('Verifying Google credentials with clinic...')
    const token = await getClerkTokenWithRetry(clerk)
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

    try {
      sessionStorage.removeItem('booka_clerk_pending_sync')
      sessionStorage.removeItem('booka_clerk_logged_out')
    } catch (e) {
      // ignore
    }

    if (onSuccess) onSuccess(data)
    return data
  } catch (err) {
    const errMsg = formatClerkError(err, 'Authentication failed')
    if (onError) onError(errMsg)
    throw err
  }
}

/**
 * Initiates Clerk authentication with Google and synchronizes the session with Booka.
 * Supports both modal presentation (bundled UI) and direct Google OAuth redirect fallback.
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
  if (typeof window !== 'undefined') {
    try {
      sessionStorage.removeItem('booka_clerk_logged_out')
    } catch (e) {
      // ignore
    }
  }

  let clerk = null
  try {
    clerk = await loadClerkSDK(clerkPublishableKey, onProgress)
  } catch (err) {
    const errMsg = formatClerkError(err, 'Failed to initialize Clerk')
    if (onError) onError(errMsg)
    throw err
  }

  // Direct Google OAuth redirect flow: sends the user straight to Google Account Chooser
  const baseReturnUrl = `${window.location.origin}${window.location.pathname}`
  const callbackUrl = `${baseReturnUrl}?sso_callback=1`
  const completeUrl = `${baseReturnUrl}?sso_complete=1`

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

  // If user already has an active Clerk session, sync immediately
  if (clerk.session) {
    return exchangeClerkSessionWithBooka(clerk, {
      tenant,
      companyName,
      onProgress,
      onSuccess,
      onError,
    })
  }

  if (onProgress) onProgress('Redirecting to Google...')

  if (mode === 'signUp' && clerk.client?.signUp?.authenticateWithRedirect) {
    await clerk.client.signUp.authenticateWithRedirect({
      strategy: 'oauth_google',
      redirectUrl: callbackUrl,
      redirectUrlComplete: completeUrl,
    })
    return { redirected: true }
  }

  if (clerk.client?.signIn?.authenticateWithRedirect) {
    await clerk.client.signIn.authenticateWithRedirect({
      strategy: 'oauth_google',
      redirectUrl: callbackUrl,
      redirectUrlComplete: completeUrl,
    })
    return { redirected: true }
  }

  // Fallback to Clerk modal only if client.authenticateWithRedirect is unavailable
  let modalOpened = false
  try {
    if (mode === 'signUp' && typeof clerk.openSignUp === 'function') {
      clerk.openSignUp({
        afterSignUpUrl: completeUrl,
        afterSignInUrl: completeUrl,
        fallbackRedirectUrl: completeUrl,
      })
      modalOpened = true
    } else if (typeof clerk.openSignIn === 'function') {
      clerk.openSignIn({
        afterSignInUrl: completeUrl,
        afterSignUpUrl: completeUrl,
        fallbackRedirectUrl: completeUrl,
      })
      modalOpened = true
    }
  } catch (uiErr) {
    console.warn('Clerk UI modal unavailable', uiErr)
    modalOpened = false
  }

  if (!modalOpened) {
    throw new Error('Clerk Google SSO method unavailable')
  }

  // Listen for session resolution inside fallback modal
  return new Promise((resolve, reject) => {
    let settled = false
    let unsubscribe = null
    let checkInterval = null
    let sawModalRendered = false

    const cleanup = () => {
      if (typeof unsubscribe === 'function') {
        try {
          unsubscribe()
        } catch (e) {
          // ignore
        }
      }
      if (checkInterval) clearInterval(checkInterval)
    }

    const completeLogin = async () => {
      if (settled) return
      settled = true
      cleanup()
      try {
        const data = await exchangeClerkSessionWithBooka(clerk, {
          tenant,
          companyName,
          onProgress,
          onSuccess,
          onError,
        })
        resolve(data)
      } catch (err) {
        reject(err)
      }
    }

    if (typeof clerk.addListener === 'function') {
      unsubscribe = clerk.addListener((resources) => {
        if (resources?.session) {
          completeLogin()
        }
      })
    }

    let attempts = 0
    checkInterval = setInterval(() => {
      attempts++
      if (clerk?.session) {
        completeLogin()
        return
      }

      if (typeof document !== 'undefined') {
        const modalEl = document.querySelector('.cl-modalBackdrop, .cl-cardBox, .cl-signIn-root, .cl-signUp-root')
        if (modalEl) {
          sawModalRendered = true
        } else if (sawModalRendered && attempts >= 3) {
          settled = true
          cleanup()
          if (onError) onError('')
          resolve(null)
          return
        }
      }

      if (attempts > 180) {
        settled = true
        cleanup()
        if (onError) onError('Authentication timed out. Please try again.')
        resolve(null)
      }
    }, 600)
  })
}

/**
 * Checks if the user has returned from a Google/Clerk OAuth redirect, handles the OAuth callback,
 * and completes Booka session sync.
 */
export async function checkAndSyncClerkSession({
  tenant = '',
  clerkPublishableKey = '',
  onProgress = null,
  onSuccess = null,
  onError = null,
} = {}) {
  if (typeof window === 'undefined') return null

  const clerkKey = normalizeClerkKey(
    clerkPublishableKey ||
      process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY ||
      window.CLERK_PUBLISHABLE_KEY
  )

  if (!clerkKey) return null

  const params = new URLSearchParams(window.location.search)
  const isSSOCallback =
    params.has('sso_callback') ||
    params.has('__clerk_status') ||
    params.has('__clerk_created_session') ||
    params.has('__clerk_handshake') ||
    window.location.hash.includes('sso-callback')
  const isSSOComplete = params.has('sso_complete')

  let pendingSync = null
  let loggedOutFlag = false
  try {
    const rawPending = sessionStorage.getItem('booka_clerk_pending_sync')
    if (rawPending) pendingSync = JSON.parse(rawPending)
    loggedOutFlag = sessionStorage.getItem('booka_clerk_logged_out') === '1'
  } catch (e) {
    // ignore
  }

  const isSessionEndedNotice = params.get('notice') === 'session_ended' || params.get('session_ended')

  // Do not auto-relogin on mount after explicit logout unless returning from an active OAuth redirect
  if ((isSessionEndedNotice || loggedOutFlag) && !pendingSync && !isSSOCallback && !isSSOComplete) {
    return null
  }

  // Only engage SDK on mount if returning from OAuth redirect or active pending flow
  if (!pendingSync && !isSSOCallback && !isSSOComplete) {
    return null
  }

  try {
    const clerk = await loadClerkSDK(clerkKey, onProgress)
    if (!clerk) return null

    const baseReturnUrl = `${window.location.origin}${window.location.pathname}`
    const completeUrl = `${baseReturnUrl}?sso_complete=1`

    // Handle returning Google OAuth callback to activate session & transfer signIn <-> signUp seamlessly
    if (isSSOCallback && !clerk.session && typeof clerk.handleRedirectCallback === 'function') {
      if (onProgress) onProgress('Completing Google sign-in...')
      await clerk.handleRedirectCallback(
        {
          afterSignInUrl: completeUrl,
          afterSignUpUrl: completeUrl,
          signInForceRedirectUrl: completeUrl,
          signUpForceRedirectUrl: completeUrl,
          signInUrl: baseReturnUrl,
          signUpUrl: baseReturnUrl,
        },
        async (to) => {
          if (clerk.session) return
          if (typeof to === 'string' && (to.includes('factor') || to.includes('verify') || to.includes('continue'))) {
            if (pendingSync?.mode === 'signUp' && typeof clerk.openSignUp === 'function') {
              clerk.openSignUp({ afterSignUpUrl: completeUrl })
            } else if (typeof clerk.openSignIn === 'function') {
              clerk.openSignIn({ afterSignInUrl: completeUrl })
            }
          }
        }
      )
    }

    if (!clerk.session) return null

    // Clean sso query params from address bar before redirecting into dashboard
    try {
      if (isSSOCallback || isSSOComplete) {
        window.history.replaceState({}, '', window.location.pathname)
      }
    } catch (e) {
      // ignore
    }

    const companyName = pendingSync?.companyName || ''
    const targetTenant = tenant || pendingSync?.tenant || ''

    return await exchangeClerkSessionWithBooka(clerk, {
      tenant: targetTenant,
      companyName,
      onProgress,
      onSuccess,
      onError,
    })
  } catch (err) {
    if (onError) onError(formatClerkError(err, 'Failed to sync Google session'))
    return null
  }
}

/**
 * Signs out active Clerk session (if initialized) when user logs out of Booka.
 */
export async function signOutClerkSession() {
  if (typeof window === 'undefined') return
  try {
    sessionStorage.setItem('booka_clerk_logged_out', '1')
    sessionStorage.removeItem('booka_clerk_pending_sync')
  } catch (e) {
    // ignore
  }

  try {
    if (window.Clerk?.session && typeof window.Clerk.signOut === 'function') {
      await window.Clerk.signOut()
      return
    }
    const clerkKey = normalizeClerkKey(process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY || window.CLERK_PUBLISHABLE_KEY)
    if (clerkKey) {
      const clerk = await loadClerkSDK(clerkKey)
      if (clerk?.session) {
        await clerk.signOut()
      }
    }
  } catch (e) {
    // non-fatal during logout
  }
}
