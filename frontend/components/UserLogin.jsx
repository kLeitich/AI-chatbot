"use client"
import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import api from '../lib/api'
import { initiateClerkGoogleAuth, checkAndSyncClerkSession } from '../lib/clerkAuth'

export default function UserLogin() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [success, setSuccess] = useState('')
  const [loading, setLoading] = useState(false)
  const router = useRouter()

  useEffect(() => {
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search)
      if (params.get('notice') === 'session_ended' || params.get('session_ended')) {
        setNotice('Your session has ended. Please sign in again.')
      }
      checkAndSyncClerkSession({
        onProgress: (status) => setSuccess(status),
        onSuccess: (data) => {
          setSuccess('Signed in successfully! Redirecting...')
          const tenantSlug = data?.tenant?.slug || (data?.user?.tenant_id ? String(data.user.tenant_id) : 'default')
          setTimeout(() => {
            router.replace(`/t/${tenantSlug}/admin/dashboard`)
          }, 700)
        },
        onError: (errMsg) => setError(errMsg),
      })
    }
  }, [router])

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    setSuccess('')
    setLoading(true)

    try {
      const res = await api.post('/auth/login', { email, password })
      const { token, role, tenant_id, redirect_url } = res.data || {}
      if (!token) {
        throw new Error('Missing authentication token in response')
      }

      localStorage.setItem('token', token)
      if (tenant_id) {
        localStorage.setItem('tenant_id', tenant_id)
      }
      if (role) {
        localStorage.setItem('role', role)
      }

      setSuccess('Login successful! Redirecting to your dashboard...')

      let target = redirect_url
      if (!target || !target.startsWith('/') || target.startsWith('//') || target.includes('://')) {
        if (role === 'platform_admin') {
          target = '/platform/dashboard'
        } else if (tenant_id) {
          target = `/t/${tenant_id}/admin/dashboard`
        } else {
          target = '/t/default/admin/dashboard'
        }
      }

      setTimeout(() => {
        router.replace(target)
      }, 700)
    } catch (err) {
      const message =
        err?.response?.data?.message ||
        err?.response?.data?.error ||
        err?.message ||
        'Invalid email or password'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleClerkSSO = async () => {
    setError('')
    setLoading(true)
    try {
      await initiateClerkGoogleAuth({
        mode: 'signIn',
        onProgress: (status) => setSuccess(status),
        onSuccess: (data) => {
          setSuccess('Signed in successfully! Redirecting...')
          const tenantSlug = data?.tenant?.slug || (data?.user?.tenant_id ? String(data.user.tenant_id) : 'default')
          setTimeout(() => {
            router.replace(`/t/${tenantSlug}/admin/dashboard`)
          }, 700)
        },
        onError: (errMsg) => {
          setError(errMsg)
          setLoading(false)
        },
      })
    } catch (err) {
      setError(err?.message || 'Failed to initiate Google / Clerk authentication')
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen grid place-items-center p-4 bg-gradient-to-br from-blue-50 to-indigo-100">
      <form
        onSubmit={submit}
        className="bg-white border rounded-2xl p-8 w-full max-w-md shadow-lg space-y-5"
      >
        <div className="space-y-2 text-center">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-2xl bg-blue-600 text-white shadow-md shadow-blue-500/25 mb-1">
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-gray-900">Sign In to Booka</h1>
          <p className="text-sm text-gray-600">
            Access your practice appointments, doctors, and dashboard
          </p>
        </div>

        {notice && (
          <div className="rounded-xl bg-amber-50 border border-amber-200 p-3 text-xs text-amber-800 font-medium">
            {notice}
          </div>
        )}

        {error && (
          <div className="rounded-xl bg-red-50 border border-red-200 p-3 text-xs text-red-700 font-medium">
            {error}
          </div>
        )}

        {success && (
          <div className="rounded-xl bg-emerald-50 border border-emerald-200 p-3 text-xs text-emerald-700 font-medium">
            {success}
          </div>
        )}

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="name@company.com"
            autoComplete="email"
            className="w-full border rounded-lg p-3 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        <div>
          <div className="flex items-center justify-between mb-1">
            <label className="block text-sm font-medium text-gray-700">Password</label>
            <Link href="/forgot-password" className="text-xs text-blue-600 hover:underline font-medium">
              Forgot password?
            </Link>
          </div>
          <input
            type="password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Enter your password"
            autoComplete="current-password"
            className="w-full border rounded-lg p-3 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        <button
          type="submit"
          disabled={loading}
          className="w-full rounded-xl bg-blue-600 px-4 py-3 text-white font-semibold transition hover:bg-blue-700 disabled:opacity-60 disabled:cursor-not-allowed"
        >
          {loading ? 'Signing in...' : 'Sign In'}
        </button>

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
          className="w-full flex items-center justify-center gap-2 rounded-xl border border-gray-300 bg-white px-4 py-2.5 text-sm font-semibold text-gray-700 transition hover:bg-gray-50 shadow-sm disabled:opacity-60"
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

        <div className="pt-2 border-t text-center space-y-2 text-sm text-gray-600">
          <div>
            Don&apos;t have an account?{' '}
            <Link href="/signup" className="text-blue-600 font-semibold hover:underline">
              Create an Account
            </Link>
          </div>
          <div>
            <Link href="/t/default" className="text-gray-500 hover:text-gray-700 hover:underline text-xs">
              Try Patient Demo Chatbot
            </Link>
          </div>
        </div>
      </form>
    </div>
  )
}
