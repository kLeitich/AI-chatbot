"use client"
import { useState } from 'react'
import { useRouter } from 'next/navigation'
import api from '../lib/api'

export default function PlatformLogin() {
  const [email, setEmail] = useState('platform@example.com')
  const [password, setPassword] = useState('platform123')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [loading, setLoading] = useState(false)
  const router = useRouter()

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    setSuccess('')
    setLoading(true)

    try {
      const res = await api.post('/auth/platform/login', { email, password })
      const token = res.data?.token
      if (!token) {
        throw new Error('Missing token')
      }
      localStorage.setItem('token', token)
      setSuccess('Login successful! Redirecting to platform dashboard...')
      setTimeout(() => {
        router.push('/platform/dashboard')
      }, 900)
    } catch (err) {
      const message = err?.response?.data?.message || err?.message || 'Login failed'
      setError(message)
      console.error('Platform login failed:', err)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen grid place-items-center p-4 bg-slate-50">
      <form onSubmit={submit} className="w-full max-w-md bg-white rounded-2xl border p-6 shadow-sm space-y-4">
        <h1 className="text-2xl font-semibold text-gray-900">Platform Admin Login</h1>
        <p className="text-sm text-gray-600">Manage all companies and users across the platform.</p>

        <label className="block text-sm font-medium text-gray-700">
          Email
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="mt-2 w-full rounded-lg border px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="platform@example.com"
          />
        </label>

        <label className="block text-sm font-medium text-gray-700">
          Password
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mt-2 w-full rounded-lg border px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="Your password"
          />
        </label>

        {error && <div className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</div>}
        {success && <div className="rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-700">{success}</div>}

        <button
          type="submit"
          disabled={loading}
          className="w-full rounded-xl bg-blue-600 px-4 py-3 text-white transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-70"
        >
          {loading ? 'Signing in...' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}
