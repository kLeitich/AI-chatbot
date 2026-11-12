"use client"
import { useEffect, useState } from 'react'
import DashboardNav from '../../../components/DashboardNav'
import api from '../../../lib/api'

export default function PlatformDashboardPage() {
  const [tab, setTab] = useState('tenants') // 'tenants' | 'users' | 'appointments' | 'audit'

  // Data states
  const [tenants, setTenants] = useState([])
  const [users, setUsers] = useState([])
  const [appointmentsData, setAppointmentsData] = useState({ appointments: [], total: 0, page: 1, limit: 20 })
  const [auditData, setAuditData] = useState({ logs: [], total: 0, page: 1, limit: 50 })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Appointment filters
  const [apTenantFilter, setApTenantFilter] = useState('')
  const [apDoctorFilter, setApDoctorFilter] = useState('')
  const [apStartFilter, setApStartFilter] = useState('')
  const [apEndFilter, setApEndFilter] = useState('')
  const [apStatusFilter, setApStatusFilter] = useState('')
  const [apPage, setApPage] = useState(1)

  // Audit filters
  const [auditActionFilter, setAuditActionFilter] = useState('')
  const [auditEntityFilter, setAuditEntityFilter] = useState('')
  const [auditPage, setAuditPage] = useState(1)

  const loadTenants = async () => {
    try {
      const res = await api.get('/platform/tenants')
      setTenants(res.data || [])
    } catch (err) {
      console.error(err)
    }
  }

  const loadUsers = async () => {
    try {
      const res = await api.get('/platform/users')
      setUsers(res.data || [])
    } catch (err) {
      console.error(err)
    }
  }

  const loadAppointments = async () => {
    try {
      const params = new URLSearchParams()
      if (apTenantFilter) params.set('tenant_id', apTenantFilter)
      if (apDoctorFilter) params.set('doctor_id', apDoctorFilter)
      if (apStartFilter) params.set('start_date', apStartFilter)
      if (apEndFilter) params.set('end_date', apEndFilter)
      if (apStatusFilter) params.set('status', apStatusFilter)
      params.set('page', String(apPage))
      params.set('limit', '20')

      const res = await api.get(`/platform/appointments?${params.toString()}`)
      setAppointmentsData(res.data || { appointments: [], total: 0, page: 1, limit: 20 })
    } catch (err) {
      console.error(err)
    }
  }

  const loadAuditLogs = async () => {
    try {
      const params = new URLSearchParams()
      if (auditActionFilter) params.set('action', auditActionFilter)
      if (auditEntityFilter) params.set('entity', auditEntityFilter)
      params.set('page', String(auditPage))
      params.set('limit', '50')

      const res = await api.get(`/platform/audit?${params.toString()}`)
      setAuditData(res.data || { logs: [], total: 0, page: 1, limit: 50 })
    } catch (err) {
      console.error(err)
    }
  }

  useEffect(() => {
    const init = async () => {
      setLoading(true)
      setError('')
      try {
        await Promise.all([loadTenants(), loadUsers(), loadAppointments(), loadAuditLogs()])
      } catch (err) {
        setError('Failed to load platform data. Please check authentication.')
      } finally {
        setLoading(false)
      }
    }
    init()
  }, [])

  useEffect(() => {
    if (tab === 'appointments') loadAppointments()
  }, [apTenantFilter, apDoctorFilter, apStartFilter, apEndFilter, apStatusFilter, apPage, tab])

  useEffect(() => {
    if (tab === 'audit') loadAuditLogs()
  }, [auditActionFilter, auditEntityFilter, auditPage, tab])

  return (
    <div className="min-h-screen bg-slate-50">
      <DashboardNav isPlatform={true} />

      <div className="mx-auto max-w-6xl p-6 space-y-6">
        <div className="rounded-2xl bg-white p-6 shadow-sm border flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-slate-900">Platform Overview (Read-Only)</h1>
            <p className="mt-1 text-sm text-slate-600">
              System-wide visibility across all clinics, appointments, users, and audit records.
            </p>
          </div>
          <span className="text-xs bg-purple-100 text-purple-800 font-semibold px-3 py-1 rounded-full uppercase tracking-wider">
            Platform Admin
          </span>
        </div>

        {error && (
          <div className="rounded-xl bg-red-50 border border-red-200 p-4 text-sm text-red-700">
            {error}
          </div>
        )}

        {/* Tab Navigation */}
        <div className="flex border-b border-slate-200 space-x-6 text-sm font-medium">
          <button
            onClick={() => setTab('tenants')}
            className={`pb-3 border-b-2 transition ${
              tab === 'tenants' ? 'border-blue-600 text-blue-600 font-semibold' : 'border-transparent text-slate-500 hover:text-slate-800'
            }`}
          >
            Clinics ({tenants.length})
          </button>
          <button
            onClick={() => setTab('users')}
            className={`pb-3 border-b-2 transition ${
              tab === 'users' ? 'border-blue-600 text-blue-600 font-semibold' : 'border-transparent text-slate-500 hover:text-slate-800'
            }`}
          >
            Users ({users.length})
          </button>
          <button
            onClick={() => setTab('appointments')}
            className={`pb-3 border-b-2 transition ${
              tab === 'appointments' ? 'border-blue-600 text-blue-600 font-semibold' : 'border-transparent text-slate-500 hover:text-slate-800'
            }`}
          >
            Appointments ({appointmentsData.total || 0})
          </button>
          <button
            onClick={() => setTab('audit')}
            className={`pb-3 border-b-2 transition ${
              tab === 'audit' ? 'border-blue-600 text-blue-600 font-semibold' : 'border-transparent text-slate-500 hover:text-slate-800'
            }`}
          >
            Audit Logs ({auditData.total || 0})
          </button>
        </div>

        {loading ? (
          <p className="text-center py-12 text-slate-500">Loading platform data...</p>
        ) : (
          <div>
            {/* Tab 1: Clinics */}
            {tab === 'tenants' && (
              <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                {tenants.map((t) => (
                  <div key={t.id} className="rounded-xl bg-white p-5 border shadow-sm space-y-3">
                    <div className="flex items-start justify-between">
                      <h3 className="font-bold text-slate-900 text-base">{t.name}</h3>
                      <span className="text-xs bg-slate-100 text-slate-700 px-2 py-0.5 rounded font-mono">
                        ID: {t.id}
                      </span>
                    </div>
                    <p className="text-xs text-slate-500 font-mono">Slug: {t.slug}</p>
                    <div className="grid grid-cols-2 gap-2 pt-2 border-t text-sm">
                      <div className="bg-slate-50 p-2 rounded">
                        <span className="block text-xs text-slate-500">Users</span>
                        <strong className="text-base text-slate-800">{t.user_count || 0}</strong>
                      </div>
                      <div className="bg-slate-50 p-2 rounded">
                        <span className="block text-xs text-slate-500">Appointments</span>
                        <strong className="text-base text-slate-800">{t.appointment_count || 0}</strong>
                      </div>
                    </div>
                    <p className="text-xs text-slate-400">Created: {new Date(t.created_at).toLocaleDateString()}</p>
                  </div>
                ))}
              </div>
            )}

            {/* Tab 2: Users */}
            {tab === 'users' && (
              <div className="overflow-x-auto rounded-xl border bg-white shadow-sm">
                <table className="min-w-full text-sm">
                  <thead className="bg-slate-100 text-slate-700">
                    <tr>
                      <th className="p-3 text-left">Email</th>
                      <th className="p-3 text-left">Role</th>
                      <th className="p-3 text-left">Status</th>
                      <th className="p-3 text-left">Clinic ID</th>
                      <th className="p-3 text-left">Joined</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200">
                    {users.map((u) => (
                      <tr key={u.id} className="hover:bg-slate-50">
                        <td className="p-3 font-medium text-slate-900">{u.email}</td>
                        <td className="p-3">
                          <span className="px-2 py-0.5 rounded text-xs font-semibold bg-slate-100 text-slate-800">
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
                        <td className="p-3 text-slate-600 font-mono text-xs">{u.tenant_id}</td>
                        <td className="p-3 text-slate-500 text-xs">{new Date(u.created_at).toLocaleDateString()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {/* Tab 3: Appointments */}
            {tab === 'appointments' && (
              <div className="space-y-4">
                <div className="rounded-xl border bg-white p-4 shadow-sm space-y-3">
                  <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                    <input
                      type="number"
                      placeholder="Clinic ID"
                      value={apTenantFilter}
                      onChange={(e) => setApTenantFilter(e.target.value)}
                      className="border rounded-lg p-2 text-xs"
                    />
                    <input
                      type="number"
                      placeholder="Doctor ID"
                      value={apDoctorFilter}
                      onChange={(e) => setApDoctorFilter(e.target.value)}
                      className="border rounded-lg p-2 text-xs"
                    />
                    <input
                      type="date"
                      value={apStartFilter}
                      onChange={(e) => setApStartFilter(e.target.value)}
                      className="border rounded-lg p-2 text-xs"
                    />
                    <input
                      type="date"
                      value={apEndFilter}
                      onChange={(e) => setApEndFilter(e.target.value)}
                      className="border rounded-lg p-2 text-xs"
                    />
                    <select
                      value={apStatusFilter}
                      onChange={(e) => setApStatusFilter(e.target.value)}
                      className="border rounded-lg p-2 text-xs bg-white"
                    >
                      <option value="">All Statuses</option>
                      <option value="pending">pending</option>
                      <option value="confirmed">confirmed</option>
                      <option value="cancelled">cancelled</option>
                    </select>
                  </div>
                </div>

                <div className="overflow-x-auto rounded-xl border bg-white shadow-sm">
                  <table className="min-w-full text-sm">
                    <thead className="bg-slate-100 text-slate-700">
                      <tr>
                        <th className="p-3 text-left">ID</th>
                        <th className="p-3 text-left">Clinic ID</th>
                        <th className="p-3 text-left">Patient</th>
                        <th className="p-3 text-left">Doctor</th>
                        <th className="p-3 text-left">Date</th>
                        <th className="p-3 text-left">Time</th>
                        <th className="p-3 text-left">Reason</th>
                        <th className="p-3 text-left">Status</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-200">
                      {appointmentsData.appointments.length === 0 ? (
                        <tr>
                          <td colSpan={8} className="p-6 text-center text-slate-500">
                            No appointments found matching filters.
                          </td>
                        </tr>
                      ) : (
                        appointmentsData.appointments.map((a) => (
                          <tr key={a.id} className="hover:bg-slate-50">
                            <td className="p-3 font-mono text-xs">{a.id}</td>
                            <td className="p-3 font-mono text-xs text-slate-600">{a.tenant_id}</td>
                            <td className="p-3 font-medium text-slate-900">{a.patient_name}</td>
                            <td className="p-3 text-slate-700">{a.doctor}</td>
                            <td className="p-3 text-slate-700 whitespace-nowrap">{a.date}</td>
                            <td className="p-3 text-slate-700 whitespace-nowrap">{a.time}</td>
                            <td className="p-3 text-slate-600 max-w-xs truncate">{a.reason}</td>
                            <td className="p-3">
                              <span className={`px-2 py-0.5 rounded text-xs font-semibold ${
                                a.status === 'confirmed' ? 'bg-green-100 text-green-800' :
                                a.status === 'cancelled' ? 'bg-red-100 text-red-800' :
                                'bg-yellow-100 text-yellow-800'
                              }`}>
                                {a.status}
                              </span>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Tab 4: Audit Logs */}
            {tab === 'audit' && (
              <div className="space-y-4">
                <div className="rounded-xl border bg-white p-4 shadow-sm flex gap-3">
                  <input
                    type="text"
                    placeholder="Filter Action (e.g. login_success, read)"
                    value={auditActionFilter}
                    onChange={(e) => setAuditActionFilter(e.target.value)}
                    className="border rounded-lg p-2 text-xs flex-1"
                  />
                  <input
                    type="text"
                    placeholder="Filter Entity (e.g. user, appointments)"
                    value={auditEntityFilter}
                    onChange={(e) => setAuditEntityFilter(e.target.value)}
                    className="border rounded-lg p-2 text-xs flex-1"
                  />
                </div>

                <div className="overflow-x-auto rounded-xl border bg-white shadow-sm">
                  <table className="min-w-full text-xs">
                    <thead className="bg-slate-100 text-slate-700">
                      <tr>
                        <th className="p-3 text-left">Timestamp</th>
                        <th className="p-3 text-left">Actor Role</th>
                        <th className="p-3 text-left">Actor ID</th>
                        <th className="p-3 text-left">Clinic ID</th>
                        <th className="p-3 text-left">Action</th>
                        <th className="p-3 text-left">Entity</th>
                        <th className="p-3 text-left">Entity ID</th>
                        <th className="p-3 text-left">IP Address</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-200">
                      {auditData.logs.length === 0 ? (
                        <tr>
                          <td colSpan={8} className="p-6 text-center text-slate-500">
                            No audit records found.
                          </td>
                        </tr>
                      ) : (
                        auditData.logs.map((l) => (
                          <tr key={l.id} className="hover:bg-slate-50">
                            <td className="p-3 font-mono whitespace-nowrap text-slate-600">
                              {new Date(l.at).toLocaleString()}
                            </td>
                            <td className="p-3">
                              <span className="font-semibold text-slate-800">{l.actor_role}</span>
                            </td>
                            <td className="p-3 font-mono text-slate-600">{l.actor_user_id || '-'}</td>
                            <td className="p-3 font-mono text-slate-600">{l.tenant_id || 'Platform'}</td>
                            <td className="p-3">
                              <span className="bg-slate-100 px-2 py-0.5 rounded font-mono text-slate-800">
                                {l.action}
                              </span>
                            </td>
                            <td className="p-3 font-medium text-slate-700">{l.entity}</td>
                            <td className="p-3 font-mono text-slate-600">{l.entity_id || '-'}</td>
                            <td className="p-3 font-mono text-slate-500">{l.ip || '-'}</td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
