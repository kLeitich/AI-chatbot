"use client"
import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import api from '../lib/api'
import { initiateClerkGoogleAuth, checkAndSyncClerkSession } from '../lib/clerkAuth'

export default function CompanySignup() {
  const [formData, setFormData] = useState({
    company_name: '',
    admin_email: '',
    admin_password: '',
    admin_password_confirm: '',
  })
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [successMessage, setSuccessMessage] = useState('')
  const router = useRouter()

  useEffect(() => {
    checkAndSyncClerkSession({
      onProgress: (status) => setSuccessMessage(status),
      onSuccess: (data) => {
        setSuccessMessage('Account and clinic practice ready! Redirecting to dashboard...')
        const tenantSlug = data?.tenant?.slug || data?.user?.tenant_id || 'default'
        setTimeout(() => {
          router.replace(`/t/${tenantSlug}/admin/dashboard`)
        }, 800)
      },
      onError: (errMsg) => setError(errMsg),
    })
  }, [router])

  const handleChange = (e) => {
    const { name, value } = e.target
    setFormData((prev) => ({ ...prev, [name]: value }))
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    // Validation
    if (!formData.company_name.trim()) {
      setError('Company name is required')
      setLoading(false)
      return
    }
    if (!formData.admin_email.trim()) {
      setError('Email is required')
      setLoading(false)
      return
    }
    if (formData.admin_password.length < 10) {
      setError('Password must be at least 10 characters long')
      setLoading(false)
      return
    }
    if (formData.admin_password !== formData.admin_password_confirm) {
      setError('Passwords do not match')
      setLoading(false)
      return
    }

    try {
      const res = await api.post('/auth/register', {
        company_name: formData.company_name,
        admin_email: formData.admin_email,
        admin_password: formData.admin_password,
      })

      const { tenant_id, token } = res.data || {}
      if (tenant_id && token) {
        localStorage.setItem('token', token)
        localStorage.setItem('tenant_id', tenant_id)
        setSuccessMessage('Registration successful! Redirecting to dashboard...')
        setTimeout(() => {
          router.replace(`/t/${tenant_id}/admin/dashboard`)
        }, 900)
        return
      }
      setError('Registration completed but no tenant or token returned.')
    } catch (e) {
      const message = e?.response?.data?.message || e?.response?.data?.error || 'Registration failed. Please try again.'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleGoogleSignUp = async () => {
    setError('')
    setSuccessMessage('')
    setLoading(true)
    try {
      const result = await initiateClerkGoogleAuth({
        mode: 'signUp',
        companyName: formData.company_name,
        onProgress: (status) => setSuccessMessage(status),
        onSuccess: (data) => {
          setSuccessMessage('Account and clinic practice ready! Redirecting to dashboard...')
          const tenantSlug = data?.tenant?.slug || data?.user?.tenant_id || 'default'
          setTimeout(() => {
            router.replace(`/t/${tenantSlug}/admin/dashboard`)
          }, 800)
        },
        onError: (errMsg) => {
          setSuccessMessage('')
          setError(errMsg)
          setLoading(false)
        },
      })
      if (!result) {
        setSuccessMessage('')
        setLoading(false)
      }
    } catch (err) {
      setSuccessMessage('')
      setError(err?.message || 'Google sign-up failed')
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen grid place-items-center p-4 bg-gray-50">
      <form onSubmit={handleSubmit} className="bg-white border rounded p-6 w-full max-w-md shadow space-y-4">
        <h2 className="text-2xl font-semibold text-gray-900">Register Your Company</h2>
        <p className="text-sm text-gray-600">Create an account to use the AI Doctor Appointment Chatbot</p>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Company Name</label>
          <input
            type="text"
            name="company_name"
            value={formData.company_name}
            onChange={handleChange}
            placeholder="Your Company Name"
            className="w-full border rounded p-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Admin Email</label>
          <input
            type="email"
            name="admin_email"
            value={formData.admin_email}
            onChange={handleChange}
            placeholder="admin@company.com"
            className="w-full border rounded p-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
          <input
            type="password"
            name="admin_password"
            value={formData.admin_password}
            onChange={handleChange}
            placeholder="At least 10 characters"
            className="w-full border rounded p-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <p className="text-xs text-gray-500 mt-1">Must be at least 10 characters long.</p>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Confirm Password</label>
          <input
            type="password"
            name="admin_password_confirm"
            value={formData.admin_password_confirm}
            onChange={handleChange}
            placeholder="Confirm your password"
            className="w-full border rounded p-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        {error && <p className="text-red-600 text-sm bg-red-50 p-3 rounded">{error}</p>}
        {successMessage && <p className="text-green-600 text-sm bg-green-50 p-3 rounded">{successMessage}</p>}

        <button
          type="submit"
          disabled={loading}
          className="w-full bg-blue-600 text-white rounded p-3 font-medium hover:bg-blue-700 disabled:opacity-60 disabled:cursor-not-allowed"
        >
          {loading ? 'Creating Account...' : 'Create Account'}
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
          onClick={handleGoogleSignUp}
          disabled={loading}
          className="w-full flex items-center justify-center gap-2 rounded-xl border border-gray-300 bg-white p-3 text-sm font-semibold text-gray-700 transition hover:bg-gray-50 shadow-sm disabled:opacity-60"
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
          Sign up with Google / Clerk SSO
        </button>

        <div className="text-center text-sm text-gray-600">
          Already have an account?{' '}
          <a href="/login" className="text-blue-600 hover:underline">
            Sign In
          </a>
        </div>
      </form>
    </div>
  )
}
