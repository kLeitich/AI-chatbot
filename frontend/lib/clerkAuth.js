import { Clerk } from '@clerk/clerk-js'
import api from './api'

const clerkInstances = {}
let inFlightSyncPromise = null

export function getPostAuthRedirectPath(data, fallbackTenant = 'default') {
  const role = data?.user?.role
  if (role === 'platform_admin') {
    return '/platform/dashboard'
  }
  const tenantSlug =
    data?.tenant?.slug ||
    (data?.user?.tenant_id ? String(data.user.tenant_id) : '') ||
    fallbackTenant ||
    'default'
  return `/t/${tenantSlug}/admin/dashboard`
}

function normalizeClerkKey(rawKey) {
  if (!rawKey || typeof rawKey !== 'string') return ''
  return rawKey.trim().replace(/^['"]|['"]$/g, '').trim()
}

function formatClerkError(err, fallbackMsg) {
  const apiMsg = err?.errors?.[0]?.longMessage || err?.errors?.[0]?.message
  const httpMsg = err?.response?.data?.message || err?.response?.data?.error
  return apiMsg || httpMsg || err?.message || fallbackMsg
}

function spaNavigate(to) {
  if (typeof window === 'undefined' || typeof to !== 'string' || !to) return
  try {
    const parsed = new URL(to, window.location.origin)
    if (parsed.origin === window.location.origin) {
      window.history.replaceState({}, '', `${parsed.pathname}${parsed.search}${parsed.hash}`)
      return
    }
    window.location.href = parsed.href
  } catch {
    // ignore invalid URL
  }
}

function describeIncompleteCallbackState(clerk) {
  const signIn = clerk?.client?.signIn
  const signUp = clerk?.client?.signUp

  const signInErr = signIn?.firstFactorVerification?.error
  const signUpErr = signUp?.verifications?.externalAccount?.error
  const oauthErr = signInErr || signUpErr
  if (oauthErr?.longMessage || oauthErr?.message) {
    return oauthErr.longMessage || oauthErr.message
  }

  if (signIn?.protectCheck || signUp?.protectCheck || signIn?.status === 'needs_protect_check') {
    return 'Additional security verification is required in Clerk Protect before signing in.'
  }

  if (signIn?.status === 'needs_second_factor' || signIn?.status === 'needs_client_trust') {
    return 'Two-factor or new-device verification is enabled for this Clerk account. Please verify or sign in with your email and password.'
  }

  if (signIn?.status === 'needs_new_password') {
    return 'This account requires a password reset before completing sign-in.'
  }

  if (signUp?.status === 'missing_requirements') {
    const missing = [...(signUp.missingFields || []), ...(signUp.unverifiedFields || [])]
    if (missing.length > 0) {
      return `Clerk sign-up requires additional fields (${missing.join(', ')}). Update Clerk Dashboard settings or register with the email form below.`
    }
    return 'Clerk sign-up requires additional verification before completion.'
  }

  return ''
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
 * Loads and initializes the bundled @clerk/clerk-js (Core 3) SDK headlessly for direct OAuth flows.
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
      await clerk.load({
        routerPush: spaNavigate,
        routerReplace: spaNavigate,
      })
    }

    if (!clerk.client || !clerk.environment) {
      const fapi = clerk.frontendApi || getClerkFrontendApi(clerkKey) || 'Clerk API'
      delete clerkInstances[clerkKey]
      throw new Error(
        `Cannot reach Clerk at https://${fapi}. Ensure DNS CNAME records for "${fapi}" are verified in Clerk Dashboard > Domains (or use a valid pk_test_ / pk_live_ key).`
      )
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
    const { token: bookaToken, user, tenant: tenantObj } = data

    if (bookaToken) {
      localStorage.setItem('token', bookaToken)
      if (user?.role) localStorage.setItem('role', user.role)
      if (user?.tenant_id) localStorage.setItem('tenant_id', String(user.tenant_id))
      if (tenantObj?.slug) localStorage.setItem('tenant', tenantObj.slug)
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
 * Initiates Clerk authentication directly with Google OAuth and synchronizes the session with Booka.
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

  try {
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
  } catch (redirectErr) {
    const fapi =
      clerk.frontendApi ||
      getClerkFrontendApi(clerkPublishableKey || process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY) ||
      'Clerk API'
    const rawMsg = formatClerkError(redirectErr, 'Failed to start Google sign-in redirect')
    const friendlyMsg =
      rawMsg.toLowerCase().includes('network') || rawMsg.toLowerCase().includes('fetch')
        ? `Cannot reach Clerk at https://${fapi}. Please verify your Clerk domain DNS CNAME records in Clerk Dashboard > Domains.`
        : rawMsg
    if (onError) onError(friendlyMsg)
    throw new Error(friendlyMsg)
  }

  // Only attempt modal UI fallback if @clerk/ui components are explicitly loaded
  const hasUiComponents = Boolean(clerk.uiVersion || clerk.components)
  if (hasUiComponents) {
    let modalOpened = false
    try {
      if (mode === 'signUp' && typeof clerk.openSignUp === 'function') {
        clerk.openSignUp({
          forceRedirectUrl: completeUrl,
          fallbackRedirectUrl: completeUrl,
        })
        modalOpened = true
      } else if (typeof clerk.openSignIn === 'function') {
        clerk.openSignIn({
          forceRedirectUrl: completeUrl,
          fallbackRedirectUrl: completeUrl,
        })
        modalOpened = true
      }
    } catch (uiErr) {
      console.warn('Clerk UI modal unavailable', uiErr)
      modalOpened = false
    }

    if (modalOpened) {
      return new Promise((resolve, reject) => {
        let settled = false
        let unsubscribe = null
        let checkInterval = null

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
          if (attempts > 180) {
            settled = true
            cleanup()
            if (onError) onError('Authentication timed out. Please try again.')
            resolve(null)
          }
        }, 600)
      })
    }
  }

  const unavailableMsg =
    'Google OAuth redirect is unavailable for this Clerk configuration. Ensure Google social sign-in is enabled in Clerk Dashboard.'
  if (onError) onError(unavailableMsg)
  throw new Error(unavailableMsg)
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
  if (inFlightSyncPromise) return inFlightSyncPromise

  const runSync = async () => {
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
      let callbackIssue = ''

      // Handle returning Google OAuth callback to activate session & transfer signIn <-> signUp seamlessly
      if (isSSOCallback && !clerk.session && typeof clerk.handleRedirectCallback === 'function') {
        if (onProgress) onProgress('Completing Google sign-in...')
        await clerk.handleRedirectCallback(
          {
            signInForceRedirectUrl: completeUrl,
            signUpForceRedirectUrl: completeUrl,
            signInFallbackRedirectUrl: completeUrl,
            signUpFallbackRedirectUrl: completeUrl,
            signInUrl: baseReturnUrl,
            signUpUrl: baseReturnUrl,
          },
          async (to) => {
            if (clerk.session) return
            const hasUiComponents = Boolean(clerk.uiVersion || clerk.components)
            if (
              hasUiComponents &&
              typeof to === 'string' &&
              (to.includes('factor') || to.includes('verify') || to.includes('continue'))
            ) {
              if (pendingSync?.mode === 'signUp' && typeof clerk.openSignUp === 'function') {
                clerk.openSignUp({ forceRedirectUrl: completeUrl, fallbackRedirectUrl: completeUrl })
                return
              }
              if (typeof clerk.openSignIn === 'function') {
                clerk.openSignIn({ forceRedirectUrl: completeUrl, fallbackRedirectUrl: completeUrl })
                return
              }
            }
            callbackIssue = describeIncompleteCallbackState(clerk) || callbackIssue
          }
        )
      }

      if (!clerk.session) {
        if (isSSOCallback || isSSOComplete) {
          try {
            window.history.replaceState({}, '', window.location.pathname)
            sessionStorage.removeItem('booka_clerk_pending_sync')
          } catch (e) {
            // ignore
          }
          const issue = callbackIssue || describeIncompleteCallbackState(clerk)
          if (issue && onError) {
            onError(issue)
          }
        }
        return null
      }

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
      try {
        sessionStorage.removeItem('booka_clerk_pending_sync')
      } catch (e) {
        // ignore
      }
      if (onError) onError(formatClerkError(err, 'Failed to sync Google session'))
      return null
    }
  }

  inFlightSyncPromise = runSync().finally(() => {
    inFlightSyncPromise = null
  })
  return inFlightSyncPromise
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
      await window.Clerk.signOut(() => {})
      return
    }
    const clerkKey = normalizeClerkKey(
      process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY || window.CLERK_PUBLISHABLE_KEY
    )
    if (clerkKey) {
      const clerk = await loadClerkSDK(clerkKey)
      if (clerk?.session) {
        await clerk.signOut(() => {})
      }
    }
  } catch (e) {
    // non-fatal during logout
  }
}
