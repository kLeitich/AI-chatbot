"use client"
import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import api from '../lib/api'
import { initiateClerkGoogleAuth } from '../lib/clerkAuth'

export default function AdminLogin({ tenant = 'default' }) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [success, setSuccess] = useState('')
  const [loading, setLoading] = useState(false)
  const [tenantConfig, setTenantConfig] = useState(null)
  const router = useRouter()

  useEffect(() => {
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search)
      if (params.get('notice') === 'session_ended' || params.get('session_ended')) {
        setNotice('Your session has ended. Please sign in again.')
      }
    }
    if (tenant) {
      api.get(`/t/${tenant}/public-config`)
        .then((res) => setTenantConfig(res.data))
        .catch(() => {})
    }
  }, [tenant])

  const submit = async (e) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    setSuccess('')
    try {
      const res = await api.post(`/t/${tenant}/login`, { email, password })
      localStorage.setItem('token', res.data?.token || '')
      setSuccess('Login successful! Redirecting to dashboard...')
      setTimeout(() => {
        router.replace(`/t/${tenant}/admin/dashboard`)
      }, 900)
    } catch (e) {
      setError('Invalid credentials')
    } finally {
      setLoading(false)
    }
  }

  const handleClerkSSO = async () => {
    setError('')
    setLoading(true)
    try {
      await initiateClerkGoogleAuth({
        tenant,
        clerkPublishableKey: tenantConfig?.clerk_publishable_key,
        onProgress: (status) => setSuccess(status),
        onSuccess: (data) => {
          setSuccess('Signed in via Clerk SSO! Redirecting...')
          setTimeout(() => {
            router.replace(`/t/${tenant}/admin/dashboard`)
          }, 700)
        },
        onError: (errMsg) => {
          setError(errMsg)
          setLoading(false)
        },
      })
    } catch (err) {
      setError(err?.message || 'Failed to initiate Clerk SSO')
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen grid place-items-center p-4">
      <form onSubmit={submit} className="bg-white border rounded p-6 w-full max-w-sm shadow space-y-3">
        <h2 className="text-xl font-semibold">{tenantConfig?.name ? `${tenantConfig.name} Admin Login` : 'Admin Login'}</h2>
        {notice && <p className="text-amber-800 bg-amber-50 border border-amber-200 rounded p-2 text-xs font-medium">{notice}</p>}
        <input value={email} onChange={(e) => setEmail(e.target.value)} type="email" placeholder="Email" className="w-full border rounded p-2" />
        <input value={password} onChange={(e) => setPassword(e.target.value)} type="password" placeholder="Password" className="w-full border rounded p-2" />
        <div className="text-right">
          <Link href="/forgot-password" className="text-xs text-blue-600 hover:underline">
            Forgot password?
          </Link>
        </div>
        {error && <p className="text-red-600 text-sm">{error}</p>}
        {success && <p className="text-green-600 text-sm">{success}</p>}
        <button className="w-full bg-blue-600 text-white rounded p-2" disabled={loading}>{loading ? 'Signing in...' : 'Sign In'}</button>

        <div className="relative my-2">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t border-gray-200" />
          </div>
          <div className="relative flex justify-center text-xs uppercase">
            <span className="bg-white px-2 text-gray-500 font-medium">Or continue with</span>
          </div>
        </div>

        <button
          type="button"
          onClick={handleClerkSSO}
          disabled={loading}
          className="w-full flex items-center justify-center gap-2 rounded border border-gray-300 bg-white p-2 text-sm font-semibold text-gray-700 transition hover:bg-gray-50 shadow-sm disabled:opacity-60"
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24">
            <path
              fill="#4285F4"
              d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
            />
            <path
              fill="#34A853"
              d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
            />
            <path
              fill="#FBBC05"
              d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"
            />
            <path
              fill="#EA4335"
              d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"
            />
          </svg>
          Sign in with Google / Clerk SSO
        </button>
      </form>
    </div>
  )
}
