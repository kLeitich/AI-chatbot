import axios from 'axios'

const api = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080',
})

api.interceptors.request.use((config) => {
  if (typeof window !== 'undefined') {
    const token = localStorage.getItem('token')
    if (token) config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Lightweight toast notification for 403 PERMISSION_DENIED
function showPermissionToast(message) {
  if (typeof window === 'undefined' || typeof document === 'undefined') return

  let toast = document.getElementById('toast-permission-denied')
  if (!toast) {
    toast = document.createElement('div')
    toast.id = 'toast-permission-denied'
    toast.className =
      'fixed top-4 right-4 z-50 flex items-center gap-2 bg-red-600 text-white text-xs font-semibold px-4 py-3 rounded-xl shadow-lg transition-all duration-300'
    document.body.appendChild(toast)
  }

  toast.textContent = message || 'Permission Denied: You do not have permission for this action.'
  toast.style.display = 'flex'
  toast.style.opacity = '1'

  if (toast.dataset.timeoutId) {
    clearTimeout(Number(toast.dataset.timeoutId))
  }

  const timeoutId = setTimeout(() => {
    toast.style.opacity = '0'
    setTimeout(() => {
      toast.style.display = 'none'
    }, 300)
  }, 4000)

  toast.dataset.timeoutId = String(timeoutId)
}

// Response interceptor
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (!error.response) {
      return Promise.reject(error)
    }

    const { status, data } = error.response
    const code = data?.code
    const message = data?.message || error.message

    if (typeof window !== 'undefined') {
      const pathname = window.location.pathname || ''
      const isLoginPage = pathname.includes('/login')
      const isLoginRequest = error.config?.url?.includes('/login')

      // On 401 (TOKEN_EXPIRED, TOKEN_REVOKED, ACCOUNT_DISABLED, or unauthorized)
      // Clear token and redirect to appropriate login page, avoiding redirect loops
      if (status === 401 && !isLoginPage && !isLoginRequest) {
        localStorage.removeItem('token')
        localStorage.removeItem('tenant_id')

        const isPlatform = pathname.startsWith('/platform') || error.config?.url?.startsWith('/platform')
        if (isPlatform) {
          window.location.href = '/platform/login?notice=session_ended'
        } else {
          const tenantMatch =
            pathname.match(/^\/t\/([^/]+)/) || (error.config?.url && error.config.url.match(/^\/t\/([^/]+)/))
          const tenant = tenantMatch ? tenantMatch[1] : (localStorage.getItem('tenant') || 'default')
          window.location.href = `/t/${tenant}/login?notice=session_ended`
        }
        return Promise.reject(error)
      }

      // On 403 PERMISSION_DENIED: show toast and DO NOT log out
      if (status === 403) {
        showPermissionToast(message || 'Permission denied. You do not have access to this resource.')
        return Promise.reject(error)
      }
    }

    return Promise.reject(error)
  }
)

export default api
export { showPermissionToast }
