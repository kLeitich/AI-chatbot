import api from './api'

/**
 * Initiates Clerk authentication with Google and synchronizes the session with Booka.
 * Supports both existing user login and new practice registration.
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

  // Load Clerk JS SDK dynamically if not already loaded
  if (typeof window !== 'undefined' && !window.Clerk) {
    if (onProgress) onProgress('Loading Google authentication...')
    await new Promise((resolve, reject) => {
      const script = document.createElement('script')
      script.src = 'https://cdn.jsdelivr.net/npm/@clerk/clerk-js@latest/dist/clerk.browser.js'
      script.async = true
      script.setAttribute('data-clerk-publishable-key', clerkKey)
      script.onload = () => resolve()
      script.onerror = () => reject(new Error('Failed to load Clerk authentication library'))
      document.head.appendChild(script)
    })
  }

  if (!window.Clerk) {
    const errorMsg = 'Clerk SDK could not be initialized'
    if (onError) onError(errorMsg)
    throw new Error(errorMsg)
  }

  if (!window.Clerk.loaded) {
    await window.Clerk.load({ publishableKey: clerkKey })
  }

  // Open Clerk Modal (Sign-Up or Sign-In)
  if (mode === 'signUp' && typeof window.Clerk.openSignUp === 'function') {
    window.Clerk.openSignUp({
      afterSignUpUrl: window.location.href,
    })
  } else if (typeof window.Clerk.openSignIn === 'function') {
    window.Clerk.openSignIn({
      afterSignInUrl: window.location.href,
    })
  }

  // Poll for active session after user authenticates with Google
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
        // Stop polling after 3 minutes
        clearInterval(checkInterval)
      }
    }, 1000)
  })
}
