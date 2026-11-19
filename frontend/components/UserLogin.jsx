"use client"
import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import api from '../lib/api'

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
    }
  }, [])

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
      if (!target) {
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

  return (
    <div className="min-h-screen grid place-items-center p-4 bg-gradient-to-br from-blue-50 to-indigo-100">
      <form
        onSubmit={submit}
        className="bg-white border rounded-2xl p-8 w-full max-w-md shadow-lg space-y-5"
      >
        <div className="space-y-1 text-center">
          <h1 className="text-2xl font-bold text-gray-900">Sign In</h1>
          <p className="text-sm text-gray-600">
            Enter your credentials to access your organization dashboard
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
          <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
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
