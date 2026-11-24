"use client"
import { useState } from 'react'
import { useSearchParams } from 'next/navigation'
import Link from 'next/link'
import api from '../lib/api'

export default function ResetPassword() {
  const searchParams = useSearchParams()
  const token = searchParams.get('token') || ''

  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    setSuccess('')

    if (!token) {
      setError('Missing or invalid password reset token. Please request a new link.')
      return
    }

    if (password.length < 10) {
      setError('Password must be at least 10 characters long.')
      return
    }

    if (password !== confirmPassword) {
      setError('Passwords do not match.')
      return
    }

    setLoading(true)
    try {
      const res = await api.post('/auth/reset-password', {
        token,
        new_password: password,
      })
      setSuccess(
        res.data?.message ||
          'Your password has been successfully reset! You can now sign in.'
      )
    } catch (err) {
      const msg =
        err?.response?.data?.message ||
        err?.response?.data?.error ||
        'Password reset link is invalid or has expired. Please request a new one.'
      setError(msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen grid place-items-center p-4 bg-gradient-to-br from-blue-50 to-indigo-100">
      <div className="bg-white border rounded-2xl p-8 w-full max-w-md shadow-lg space-y-6">
        <div className="text-center space-y-2">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-2xl bg-blue-600 text-white shadow-md shadow-blue-500/25 mb-1">
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-gray-900">Set New Password</h1>
          <p className="text-sm text-gray-600">
            Choose a strong password with at least 10 characters.
          </p>
        </div>

        {!token && (
          <div className="rounded-xl bg-amber-50 border border-amber-200 p-4 text-xs text-amber-800 font-medium">
            No reset token detected in the link. Please make sure you clicked the full link from your email, or{' '}
            <Link href="/forgot-password" className="text-blue-600 underline font-semibold">
              request a new reset link
            </Link>
            .
          </div>
        )}

        {error && (
          <div className="rounded-xl bg-red-50 border border-red-200 p-3 text-xs text-red-700 font-medium">
            {error}
          </div>
        )}

        {success ? (
          <div className="space-y-4">
            <div className="rounded-xl bg-emerald-50 border border-emerald-200 p-4 text-sm text-emerald-800 font-medium leading-relaxed">
              {success}
            </div>
            <div className="pt-2 text-center">
              <Link
                href="/login"
                className="inline-block w-full py-3 bg-blue-600 text-white font-semibold text-sm rounded-xl hover:bg-blue-700 transition"
              >
                Go to Sign In
              </Link>
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                New Password
              </label>
              <input
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="At least 10 characters"
                className="w-full border rounded-lg p-3 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <p className="text-xs text-gray-500 mt-1">
                Must be at least 10 characters with uppercase, lowercase, number, and special character.
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Confirm New Password
              </label>
              <input
                type="password"
                required
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Confirm your new password"
                className="w-full border rounded-lg p-3 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>

            <button
              type="submit"
              disabled={loading || !token}
              className="w-full bg-blue-600 text-white rounded-lg p-3 text-sm font-semibold hover:bg-blue-700 transition disabled:opacity-60 disabled:cursor-not-allowed"
            >
              {loading ? 'Resetting password...' : 'Update Password'}
            </button>

            <div className="text-center pt-2">
              <Link
                href="/login"
                className="text-xs font-semibold text-gray-600 hover:text-blue-600 transition"
              >
                ← Back to Login
              </Link>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
