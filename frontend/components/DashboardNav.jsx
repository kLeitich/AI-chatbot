"use client"
import { useState } from 'react'
import { useRouter } from 'next/navigation'
import api from '../lib/api'

export default function DashboardNav({ tenant, isPlatform = false }) {
  const router = useRouter()
  const [showMenu, setShowMenu] = useState(false)
  const [showPasswordModal, setShowPasswordModal] = useState(false)
  const [showAddUserModal, setShowAddUserModal] = useState(false)
  const [passwordData, setPasswordData] = useState({ currentPassword: '', newPassword: '', confirmPassword: '' })
  const [newUserData, setNewUserData] = useState({ email: '', password: '', role: 'user' })
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [loading, setLoading] = useState(false)

  const handleLogout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('tenant_id')
    router.replace(isPlatform ? '/platform/login' : `/t/${tenant}/login`)
  }

  const handleChangePassword = async () => {
    setError('')
    setSuccess('')
    if (!passwordData.currentPassword || !passwordData.newPassword) {
      setError('Current and new password required')
      return
    }
    if (passwordData.newPassword !== passwordData.confirmPassword) {
      setError('New passwords do not match')
      return
    }
    if (passwordData.newPassword.length < 6) {
      setError('New password must be at least 6 characters')
      return
    }

    setLoading(true)
    try {
      const endpoint = isPlatform ? '/auth/change-password' : `/t/${tenant}/change-password`
      await api.post(endpoint, {
        current_password: passwordData.currentPassword,
        new_password: passwordData.newPassword,
      })
      setSuccess('Password changed successfully!')
      setPasswordData({ currentPassword: '', newPassword: '', confirmPassword: '' })
      setTimeout(() => setShowPasswordModal(false), 1500)
    } catch (err) {
      setError(err?.response?.data?.message || 'Failed to change password')
    } finally {
      setLoading(false)
    }
  }

  const handleAddUser = async () => {
    setError('')
    setSuccess('')
    if (!newUserData.email || !newUserData.password) {
      setError('Email and password required')
      return
    }
    if (newUserData.password.length < 6) {
      setError('Password must be at least 6 characters')
      return
    }

    setLoading(true)
    try {
      const endpoint = isPlatform ? '/platform/users' : `/t/${tenant}/admin/users`
      await api.post(endpoint, {
        email: newUserData.email,
        password: newUserData.password,
        role: newUserData.role,
      })
      setSuccess('User added successfully!')
      setNewUserData({ email: '', password: '', role: 'user' })
      setTimeout(() => setShowAddUserModal(false), 1500)
    } catch (err) {
      setError(err?.response?.data?.message || 'Failed to add user')
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <nav className="bg-white border-b shadow-sm sticky top-0 z-40">
        <div className="max-w-6xl mx-auto px-4 py-3 flex items-center justify-between">
          <h1 className="text-xl font-semibold text-slate-900">
            {isPlatform ? 'Platform Dashboard' : 'Admin Dashboard'}
          </h1>
          <div className="relative">
            <button
              onClick={() => setShowMenu(!showMenu)}
              className="px-4 py-2 rounded bg-slate-200 text-slate-900 hover:bg-slate-300 text-sm font-medium"
            >
              Profile ▼
            </button>
            {showMenu && (
              <div className="absolute right-0 mt-2 w-48 bg-white border rounded-lg shadow-lg overflow-hidden z-50">
                <button
                  onClick={() => {
                    setShowPasswordModal(true)
                    setShowMenu(false)
                  }}
                  className="w-full text-left px-4 py-2 hover:bg-slate-50 text-sm"
                >
                  Edit Password
                </button>
                <button
                  onClick={() => {
                    setShowAddUserModal(true)
                    setShowMenu(false)
                  }}
                  className="w-full text-left px-4 py-2 hover:bg-slate-50 text-sm border-t"
                >
                  Add User
                </button>
                <button
                  onClick={() => {
                    handleLogout()
                    setShowMenu(false)
                  }}
                  className="w-full text-left px-4 py-2 hover:bg-red-50 text-red-600 text-sm border-t"
                >
                  Log Out
                </button>
              </div>
            )}
          </div>
        </div>
      </nav>

      {showPasswordModal && (
        <div className="fixed inset-0 bg-black/40 grid place-items-center p-4 z-50">
          <div className="bg-white rounded-lg shadow-lg w-full max-w-md p-6 space-y-4">
            <h2 className="text-xl font-semibold text-slate-900">Change Password</h2>
            {error && <div className="text-sm text-red-600 bg-red-50 p-3 rounded">{error}</div>}
            {success && <div className="text-sm text-green-600 bg-green-50 p-3 rounded">{success}</div>}
            <div className="space-y-3">
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Current Password</label>
                <input
                  type="password"
                  value={passwordData.currentPassword}
                  onChange={(e) => setPasswordData({ ...passwordData, currentPassword: e.target.value })}
                  className="w-full border rounded px-3 py-2 text-sm"
                  placeholder="••••••"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">New Password</label>
                <input
                  type="password"
                  value={passwordData.newPassword}
                  onChange={(e) => setPasswordData({ ...passwordData, newPassword: e.target.value })}
                  className="w-full border rounded px-3 py-2 text-sm"
                  placeholder="••••••"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Confirm Password</label>
                <input
                  type="password"
                  value={passwordData.confirmPassword}
                  onChange={(e) => setPasswordData({ ...passwordData, confirmPassword: e.target.value })}
                  className="w-full border rounded px-3 py-2 text-sm"
                  placeholder="••••••"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setShowPasswordModal(false)}
                className="px-4 py-2 rounded border text-sm hover:bg-slate-50"
              >
                Cancel
              </button>
              <button
                onClick={handleChangePassword}
                disabled={loading}
                className="px-4 py-2 rounded bg-blue-600 text-white text-sm hover:bg-blue-700 disabled:opacity-60"
              >
                {loading ? 'Updating...' : 'Update Password'}
              </button>
            </div>
          </div>
        </div>
      )}

      {showAddUserModal && (
        <div className="fixed inset-0 bg-black/40 grid place-items-center p-4 z-50">
          <div className="bg-white rounded-lg shadow-lg w-full max-w-md p-6 space-y-4">
            <h2 className="text-xl font-semibold text-slate-900">
              {isPlatform ? 'Create Platform User' : 'Invite User to Organization'}
            </h2>
            {error && <div className="text-sm text-red-600 bg-red-50 p-3 rounded">{error}</div>}
            {success && <div className="text-sm text-green-600 bg-green-50 p-3 rounded">{success}</div>}
            <div className="space-y-3">
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Email</label>
                <input
                  type="email"
                  value={newUserData.email}
                  onChange={(e) => setNewUserData({ ...newUserData, email: e.target.value })}
                  className="w-full border rounded px-3 py-2 text-sm"
                  placeholder="user@example.com"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Password</label>
                <input
                  type="password"
                  value={newUserData.password}
                  onChange={(e) => setNewUserData({ ...newUserData, password: e.target.value })}
                  className="w-full border rounded px-3 py-2 text-sm"
                  placeholder="••••••"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Role</label>
                <select
                  value={newUserData.role}
                  onChange={(e) => setNewUserData({ ...newUserData, role: e.target.value })}
                  className="w-full border rounded px-3 py-2 text-sm"
                >
                  {isPlatform ? (
                    <>
                      <option value="user">User</option>
                      <option value="platform_admin">Platform Admin</option>
                    </>
                  ) : (
                    <>
                      <option value="user">User</option>
                      <option value="tenant_admin">Admin</option>
                    </>
                  )}
                </select>
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setShowAddUserModal(false)}
                className="px-4 py-2 rounded border text-sm hover:bg-slate-50"
              >
                Cancel
              </button>
              <button
                onClick={handleAddUser}
                disabled={loading}
                className="px-4 py-2 rounded bg-green-600 text-white text-sm hover:bg-green-700 disabled:opacity-60"
              >
                {loading ? 'Adding...' : 'Add User'}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
