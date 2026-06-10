"use client"
import { useState } from 'react'
import { useRouter } from 'next/navigation'
import api from '../lib/api'

export default function CompanySignup() {
  const [formData, setFormData] = useState({
    company_name: '',
    admin_email: '',
    admin_password: '',
    admin_password_confirm: '',
  })
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [success, setSuccess] = useState(false)
  const router = useRouter()

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
    if (formData.admin_password.length < 6) {
      setError('Password must be at least 6 characters')
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
        setSuccess(true)
        // Redirect to admin dashboard after 2 seconds
        setTimeout(() => {
          router.replace(`/t/${tenant_id}/admin/dashboard`)
        }, 2000)
      }
    } catch (e) {
      const message = e?.response?.data?.error || 'Registration failed. Please try again.'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  if (success) {
    return (
      <div className="min-h-screen grid place-items-center p-4">
        <div className="bg-green-50 border border-green-200 rounded p-6 w-full max-w-sm text-center space-y-3">
          <h2 className="text-xl font-semibold text-green-800">Registration Successful!</h2>
          <p className="text-green-700">Your company has been registered. Redirecting to dashboard...</p>
        </div>
      </div>
    )
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
            placeholder="At least 6 characters"
            className="w-full border rounded p-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
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

        <button
          type="submit"
          disabled={loading}
          className="w-full bg-blue-600 text-white rounded p-3 font-medium hover:bg-blue-700 disabled:opacity-60 disabled:cursor-not-allowed"
        >
          {loading ? 'Creating Account...' : 'Create Account'}
        </button>

        <div className="text-center text-sm text-gray-600">
          Already have an account?{' '}
          <a href="/t/default/login" className="text-blue-600 hover:underline">
            Sign In
          </a>
        </div>
      </form>
    </div>
  )
}
