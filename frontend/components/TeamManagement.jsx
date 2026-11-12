"use client"
import { useEffect, useState } from 'react'
import api from '../lib/api'

const AVAILABLE_PERMISSIONS = [
  'appointments.view',
  'appointments.create',
  'appointments.edit',
  'appointments.cancel',
  'appointments.delete',
  'doctors.view',
  'doctors.manage',
  'reasons.manage',
  'settings.manage',
]

export default function TeamManagement({ tenant }) {
  const [users, setUsers] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [tempPasswordNotice, setTempPasswordNotice] = useState(null)

  // Add user modal state
  const [showAddModal, setShowAddModal] = useState(false)
  const [addEmail, setAddEmail] = useState('')
  const [addName, setAddName] = useState('')
  const [addPreset, setAddPreset] = useState('receptionist')
  const [customPerms, setCustomPerms] = useState([])
  const [addPassword, setAddPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const loadUsers = async () => {
    setLoading(true)
    setError('')
    try {
      const res = await api.get(`/t/${tenant}/admin/users`)
      setUsers(res.data || [])
    } catch (e) {
      setError(e?.response?.data?.message || 'Failed to load team members')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadUsers()
  }, [tenant])

  const handleToggleStatus = async (user) => {
    const newStatus = user.status === 'active' ? 'disabled' : 'active'
    const confirmMsg = newStatus === 'disabled'
      ? `Are you sure you want to disable ${user.email}? They will immediately lose access.`
      : `Enable account for ${user.email}?`
    if (!confirm(confirmMsg)) return

    try {
      await api.patch(`/t/${tenant}/admin/users/${user.id}/status`, { status: newStatus })
      await loadUsers()
    } catch (e) {
      alert(e?.response?.data?.message || 'Failed to update user status')
    }
  }

  const handleResetPassword = async (user) => {
    if (!confirm(`Reset password for ${user.email}? A temporary password will be generated.`)) return

    try {
      const res = await api.post(`/t/${tenant}/admin/users/${user.id}/reset-password`, {})
      setTempPasswordNotice({
        email: user.email,
        password: res.data?.temporary_password,
        action: 'reset',
      })
      await loadUsers()
    } catch (e) {
      alert(e?.response?.data?.message || 'Failed to reset password')
    }
  }

  const handleAddUserSubmit = async (e) => {
    e.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      const payload = {
        email: addEmail,
        name: addName,
      }
      if (addPreset === 'custom') {
        payload.permissions = customPerms
      } else {
        payload.preset = addPreset
      }
      if (addPassword.trim()) {
        payload.password = addPassword.trim()
      }

      const res = await api.post(`/t/${tenant}/admin/users`, payload)
      setTempPasswordNotice({
        email: res.data?.email || addEmail,
        password: res.data?.temporary_password,
        action: 'create',
      })
      setShowAddModal(false)
      setAddEmail('')
      setAddName('')
      setAddPassword('')
      setAddPreset('receptionist')
      setCustomPerms([])
      await loadUsers()
    } catch (e) {
      alert(e?.response?.data?.message || 'Failed to create user')
    } finally {
      setSubmitting(false)
    }
  }

  const toggleCustomPerm = (perm) => {
    if (customPerms.includes(perm)) {
      setCustomPerms(customPerms.filter((p) => p !== perm))
    } else {
      setCustomPerms([...customPerms, perm])
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold text-slate-900">Clinic Team & Permissions</h2>
          <p className="text-sm text-slate-500">Manage clinic staff, presets, and access privileges.</p>
        </div>
        <button
          onClick={() => setShowAddModal(true)}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg shadow-sm"
        >
          + Add Staff Member
        </button>
      </div>

      {tempPasswordNotice && (
        <div className="bg-amber-50 border border-amber-300 rounded-xl p-4 text-amber-900 flex items-start justify-between">
          <div>
            <h4 className="font-semibold text-amber-900">Temporary Password Generated</h4>
            <p className="text-sm mt-1">
              For account: <strong className="font-mono">{tempPasswordNotice.email}</strong>
            </p>
            <p className="text-sm mt-1">
              Temporary Password: <strong className="bg-white px-2 py-0.5 border border-amber-200 rounded font-mono text-base">{tempPasswordNotice.password}</strong>
            </p>
            <p className="text-xs text-amber-700 mt-2">
              Please copy this password and securely share it with the team member. They will be required to change it on their next login.
            </p>
          </div>
          <button
            onClick={() => setTempPasswordNotice(null)}
            className="text-amber-700 hover:text-amber-900 font-bold ml-4"
          >
            ✕
          </button>
        </div>
      )}

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg text-sm">
          {error}
        </div>
      )}

      {loading ? (
        <p className="text-gray-500 py-6 text-center">Loading team members...</p>
      ) : (
        <div className="overflow-x-auto border rounded-xl bg-white shadow-sm">
          <table className="min-w-full text-sm">
            <thead className="bg-slate-100 text-slate-700">
              <tr>
                <th className="p-3 text-left">User</th>
                <th className="p-3 text-left">Role</th>
                <th className="p-3 text-left">Status</th>
                <th className="p-3 text-left">Permissions</th>
                <th className="p-3 text-left">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {users.map((u) => (
                <tr key={u.id} className="hover:bg-slate-50">
                  <td className="p-3">
                    <div className="font-medium text-slate-900">{u.email}</div>
                    {u.must_change_password && (
                      <span className="text-xs text-amber-600 bg-amber-50 border border-amber-200 px-1.5 py-0.5 rounded">
                        Must change password
                      </span>
                    )}
                  </td>
                  <td className="p-3">
                    <span className={`px-2 py-0.5 rounded text-xs font-semibold ${
                      u.role === 'tenant_admin' ? 'bg-purple-100 text-purple-800' : 'bg-slate-100 text-slate-800'
                    }`}>
                      {u.role}
                    </span>
                  </td>
                  <td className="p-3">
                    <span className={`px-2 py-0.5 rounded text-xs font-semibold ${
                      u.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
                    }`}>
                      {u.status}
                    </span>
                  </td>
                  <td className="p-3 max-w-xs">
                    {u.role === 'tenant_admin' ? (
                      <span className="text-xs text-slate-500 italic">Full Clinic Admin Access</span>
                    ) : (
                      <div className="flex flex-wrap gap-1">
                        {(u.permissions || []).map((p) => (
                          <span key={p} className="bg-blue-50 text-blue-700 text-xs px-1.5 py-0.5 rounded border border-blue-200">
                            {p}
                          </span>
                        ))}
                      </div>
                    )}
                  </td>
                  <td className="p-3">
                    <div className="flex gap-2">
                      {u.role !== 'tenant_admin' && (
                        <button
                          onClick={() => handleToggleStatus(u)}
                          className={`px-2.5 py-1 rounded text-xs font-medium text-white ${
                            u.status === 'active' ? 'bg-gray-600 hover:bg-gray-700' : 'bg-green-600 hover:bg-green-700'
                          }`}
                        >
                          {u.status === 'active' ? 'Disable' : 'Enable'}
                        </button>
                      )}
                      <button
                        onClick={() => handleResetPassword(u)}
                        className="px-2.5 py-1 rounded text-xs font-medium bg-amber-600 hover:bg-amber-700 text-white"
                      >
                        Reset Password
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showAddModal && (
        <div className="fixed inset-0 bg-black/50 grid place-items-center p-4 z-50">
          <div className="bg-white rounded-2xl shadow-xl w-full max-w-lg p-6 space-y-4">
            <h3 className="text-lg font-semibold text-slate-900">Add Staff Member</h3>
            <form onSubmit={handleAddUserSubmit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Email Address *</label>
                <input
                  type="email"
                  required
                  value={addEmail}
                  onChange={(e) => setAddEmail(e.target.value)}
                  className="w-full border rounded-lg p-2.5 text-sm"
                  placeholder="staff@clinic.com"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Name (Optional)</label>
                <input
                  type="text"
                  value={addName}
                  onChange={(e) => setAddName(e.target.value)}
                  className="w-full border rounded-lg p-2.5 text-sm"
                  placeholder="Jane Wanjiku"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Role Preset</label>
                <select
                  value={addPreset}
                  onChange={(e) => setAddPreset(e.target.value)}
                  className="w-full border rounded-lg p-2.5 text-sm bg-white"
                >
                  <option value="viewer">Viewer (View appointments & doctors)</option>
                  <option value="receptionist">Receptionist (Book, edit, cancel appointments)</option>
                  <option value="manager">Manager (All appointment & doctor management)</option>
                  <option value="custom">Custom Permissions</option>
                </select>
              </div>

              {addPreset === 'custom' && (
                <div className="border rounded-lg p-3 bg-slate-50 space-y-2">
                  <span className="text-xs font-semibold text-slate-600 block">Select Permissions:</span>
                  <div className="grid grid-cols-2 gap-2">
                    {AVAILABLE_PERMISSIONS.map((perm) => (
                      <label key={perm} className="flex items-center gap-2 text-xs text-slate-700 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={customPerms.includes(perm)}
                          onChange={() => toggleCustomPerm(perm)}
                        />
                        <span>{perm}</span>
                      </label>
                    ))}
                  </div>
                </div>
              )}

              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">
                  Temporary Password (Leave blank to auto-generate)
                </label>
                <input
                  type="text"
                  value={addPassword}
                  onChange={(e) => setAddPassword(e.target.value)}
                  className="w-full border rounded-lg p-2.5 text-sm font-mono"
                  placeholder="Leave empty for secure generated password"
                />
              </div>

              <div className="flex justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddModal(false)}
                  className="px-4 py-2 border rounded-lg text-sm text-slate-700 hover:bg-slate-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium"
                >
                  {submitting ? 'Creating...' : 'Create Staff Member'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

