"use client"
import { useEffect, useMemo, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import api from '../lib/api'
import { usePermissions } from '../context/PermissionContext'
import AppointmentTable from './AppointmentTable'
import Calendar from './Calendar'
import DashboardNav from './DashboardNav'
import TeamManagement from './TeamManagement'
import ClinicSetup from './ClinicSetup'
import AuditLogTable from './AuditLogTable'

export default function AdminDashboard({ tenant = 'default' }) {
  const permCtx = usePermissions()
  const [appointments, setAppointments] = useState([])
  const [editing, setEditing] = useState(null)
  const [loading, setLoading] = useState(true)
  const [view, setView] = useState('table')
  const [calendarMode, setCalendarMode] = useState('month')
  const [currentDate, setCurrentDate] = useState(new Date())
  const router = useRouter()

  // Modal helpers for doctors, availability, and conflict alternatives
  const [activeDoctors, setActiveDoctors] = useState([])
  const [availableSlots, setAvailableSlots] = useState([])
  const [loadingSlots, setLoadingSlots] = useState(false)
  const [modalError, setModalError] = useState('')
  const [conflictAlternatives, setConflictAlternatives] = useState([])

  const todayInNairobi = useMemo(() => {
    try {
      return new Intl.DateTimeFormat('en-CA', {
        timeZone: 'Africa/Nairobi',
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
      }).format(new Date())
    } catch {
      return new Date().toISOString().slice(0, 10)
    }
  }, [])

  const loadDoctors = useCallback(async () => {
    try {
      const res = await api.get(`/t/${tenant}/doctors`)
      setActiveDoctors(res.data || [])
    } catch (e) {
      console.error('Failed to load doctors:', e)
    }
  }, [tenant])

  useEffect(() => {
    loadDoctors()
  }, [loadDoctors])

  useEffect(() => {
    if (!editing || !editing.doctor || !editing.date) {
      setAvailableSlots([])
      return
    }
    let cancelled = false
    const fetchSlots = async () => {
      setLoadingSlots(true)
      try {
        const res = await api.get(`/t/${tenant}/availability`, {
          params: { doctor: editing.doctor, date: editing.date },
        })
        if (!cancelled) {
          setAvailableSlots(res.data?.slots || [])
        }
      } catch (err) {
        if (!cancelled) setAvailableSlots([])
      } finally {
        if (!cancelled) setLoadingSlots(false)
      }
    }
    fetchSlots()
    return () => {
      cancelled = true
    }
  }, [editing?.doctor, editing?.date, tenant])

  // User profile & permissions from /t/:tenant/me
  const [localUserProfile, setLocalUserProfile] = useState(null)
  const [forcePasswordChange, setForcePasswordChange] = useState(false)
  const [forceCurrentPassword, setForceCurrentPassword] = useState('')
  const [forceNewPassword, setForceNewPassword] = useState('')
  const [forceConfirmPassword, setForceConfirmPassword] = useState('')
  const [forcePasswordError, setForcePasswordError] = useState('')
  const [forcePasswordSuccess, setForcePasswordSuccess] = useState('')

  // Active user profile: from context if available, else local
  const userProfile = permCtx.user || localUserProfile

  // Permission helper: uses context `can` or computes locally
  const can = useCallback(
    (permission) => {
      if (permCtx.user) {
        return permCtx.can(permission)
      }
      if (!userProfile) return false
      if (userProfile.role === 'tenant_admin') return true
      return (userProfile.permissions || []).includes(permission)
    },
    [permCtx, userProfile]
  )

  // Filters
  const [fPatient, setFPatient] = useState('')
  const [fDoctor, setFDoctor] = useState('')
  const [fDate, setFDate] = useState('')
  const [fTime, setFTime] = useState('')
  const [fReason, setFReason] = useState('')
  const [fStatus, setFStatus] = useState('')

  // Check /me and load appointments
  const loadProfileAndData = useCallback(async () => {
    setLoading(true)
    try {
      let profile = permCtx.user
      if (!profile) {
        const meRes = await api.get(`/t/${tenant}/me`)
        profile = meRes.data
        setLocalUserProfile(profile)
      }

      if (profile.must_change_password) {
        setForcePasswordChange(true)
      }

      // Check if user has permission to view appointments
      const canView = profile.role === 'tenant_admin' || (profile.permissions || []).includes('appointments.view')
      if (canView) {
        const res = await api.get(`/t/${tenant}/admin/appointments`)
        setAppointments(res.data || [])
      } else {
        setAppointments([])
      }
    } catch (e) {
      if (e?.response?.status === 401) {
        localStorage.removeItem('token')
        router.replace(`/t/${tenant}/login`)
      }
    } finally {
      setLoading(false)
    }
  }, [tenant, permCtx.user, router])

  useEffect(() => {
    loadProfileAndData()
  }, [loadProfileAndData])

  // Permission flags
  const isTenantAdmin = userProfile?.role === 'tenant_admin'
  const canViewAppointments = can('appointments.view')
  const canCreateAppointments = can('appointments.create')
  const canEditAppointments = can('appointments.edit')
  const canCancelAppointments = can('appointments.cancel')
  const canDeleteAppointments = can('appointments.delete')
  const canAccessSetup = can('doctors.manage') || can('reasons.manage') || can('settings.manage')

  const filtered = useMemo(() => {
    const match = (val, q) => String(val || '').toLowerCase().includes(String(q || '').toLowerCase())
    return (appointments || []).filter((a) =>
      match(a.patient_name, fPatient) &&
      match(a.doctor, fDoctor) &&
      (fDate ? String(a.date || '').startsWith(fDate) : true) &&
      match(a.time, fTime) &&
      match(a.reason, fReason) &&
      (fStatus ? String(a.status || '') === fStatus : true)
    )
  }, [appointments, fPatient, fDoctor, fDate, fTime, fReason, fStatus])

  const clearFilters = () => {
    setFPatient(''); setFDoctor(''); setFDate(''); setFTime(''); setFReason(''); setFStatus('')
  }

  const onCancel = async (a) => {
    if (!confirm(`Cancel appointment #${a.id}?`)) return
    try {
      await api.put(`/t/${tenant}/admin/appointments/${a.id}`, { status: 'cancelled' })
      loadProfileAndData()
    } catch (e) {
      alert(e?.response?.data?.message || 'Failed to cancel appointment')
    }
  }

  const onDelete = async (a) => {
    if (!confirm(`Delete appointment #${a.id}?`)) return
    try {
      await api.delete(`/t/${tenant}/admin/appointments/${a.id}`)
      loadProfileAndData()
    } catch (e) {
      alert(e?.response?.data?.message || 'Failed to delete appointment')
    }
  }

  const onSave = async () => {
    setModalError('')
    setConflictAlternatives([])
    if (!editing?.patient_name?.trim() || !editing?.doctor?.trim() || !editing?.date || !editing?.time) {
      setModalError('Please fill in patient name, doctor, date, and time.')
      return
    }
    if (editing.date < todayInNairobi) {
      setModalError('Cannot book an appointment in the past.')
      return
    }

    const { id, ...rest } = editing
    try {
      if (id) {
        await api.put(`/t/${tenant}/admin/appointments/${id}`, rest)
      } else {
        await api.post(`/t/${tenant}/admin/appointments`, rest)
      }
      setEditing(null)
      setModalError('')
      setConflictAlternatives([])
      await loadProfileAndData()
    } catch (e) {
      const code = e?.response?.data?.code
      const msg = e?.response?.data?.message || 'Failed to save appointment'
      const alts = e?.response?.data?.alternatives || []

      setModalError(msg)
      if (code === 'SLOT_TAKEN' || e?.response?.status === 409) {
        setConflictAlternatives(alts)
      }
    }
  }

  const onAddFromCalendar = (draft) => {
    setModalError('')
    setConflictAlternatives([])
    setEditing({ ...draft })
  }
  const onAddFromTable = () => {
    setModalError('')
    setConflictAlternatives([])
    setEditing({ patient_name: '', doctor: '', date: '', time: '', reason: '', status: 'pending' })
  }

  const handleForcedPasswordSubmit = async (e) => {
    e.preventDefault()
    setForcePasswordError('')
    setForcePasswordSuccess('')

    if (forceNewPassword !== forceConfirmPassword) {
      setForcePasswordError('New passwords do not match')
      return
    }
    if (forceNewPassword.length < 10) {
      setForcePasswordError('Password must be at least 10 characters long')
      return
    }

    try {
      await api.post(`/t/${tenant}/change-password`, {
        current_password: forceCurrentPassword,
        new_password: forceNewPassword,
      })
      setForcePasswordSuccess('Password updated successfully! Redirecting...')
      setTimeout(() => {
        setForcePasswordChange(false)
        loadProfileAndData()
      }, 1500)
    } catch (err) {
      setForcePasswordError(err?.response?.data?.message || 'Failed to change password')
    }
  }

  return (
    <div className="min-h-screen bg-slate-50">
      <DashboardNav tenant={tenant} isPlatform={false} />

      {forcePasswordChange && (
        <div className="fixed inset-0 bg-slate-900/90 z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl shadow-2xl max-w-md w-full p-6 space-y-4">
            <div className="border-b pb-3">
              <h2 className="text-xl font-bold text-slate-900">Password Change Required</h2>
              <p className="text-sm text-slate-600 mt-1">
                You must update your temporary password before accessing the system.
              </p>
            </div>

            {forcePasswordError && (
              <div className="bg-red-50 border border-red-200 text-red-700 text-sm p-3 rounded-lg">
                {forcePasswordError}
              </div>
            )}
            {forcePasswordSuccess && (
              <div className="bg-green-50 border border-green-200 text-green-700 text-sm p-3 rounded-lg">
                {forcePasswordSuccess}
              </div>
            )}

            <form onSubmit={handleForcedPasswordSubmit} className="space-y-3">
              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">Current / Temp Password</label>
                <input
                  type="password"
                  required
                  value={forceCurrentPassword}
                  onChange={(e) => setForceCurrentPassword(e.target.value)}
                  className="w-full border rounded-lg p-2 text-sm"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">New Password (min 10 chars)</label>
                <input
                  type="password"
                  required
                  value={forceNewPassword}
                  onChange={(e) => setForceNewPassword(e.target.value)}
                  className="w-full border rounded-lg p-2 text-sm"
                  placeholder="Min 10 characters"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">Confirm New Password</label>
                <input
                  type="password"
                  required
                  value={forceConfirmPassword}
                  onChange={(e) => setForceConfirmPassword(e.target.value)}
                  className="w-full border rounded-lg p-2 text-sm"
                />
              </div>

              <button
                type="submit"
                className="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium py-2.5 rounded-lg text-sm mt-2"
              >
                Change Password & Continue
              </button>
            </form>
          </div>
        </div>
      )}

      <div className="max-w-5xl mx-auto p-4 space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {view !== 'team' && view !== 'setup' && view !== 'audit' && canCreateAppointments && (
              <button className="px-3 py-1 rounded bg-blue-600 text-white font-medium text-sm" onClick={onAddFromTable}>
                + New Appointment
              </button>
            )}

            <div className="inline-flex border rounded overflow-hidden shadow-sm">
              {canViewAppointments && (
                <>
                  <button className={`px-3 py-1 text-sm ${view === 'table' ? 'bg-blue-600 text-white' : 'bg-white text-slate-700'}`} onClick={() => setView('table')}>Table</button>
                  <button className={`px-3 py-1 text-sm ${view === 'calendar' ? 'bg-blue-600 text-white' : 'bg-white text-slate-700'}`} onClick={() => setView('calendar')}>Calendar</button>
                </>
              )}
              {canAccessSetup && (
                <button className={`px-3 py-1 text-sm font-medium ${view === 'setup' ? 'bg-blue-600 text-white' : 'bg-white text-slate-700'}`} onClick={() => setView('setup')}>Clinic Setup</button>
              )}
              {isTenantAdmin && (
                <>
                  <button className={`px-3 py-1 text-sm font-medium ${view === 'team' ? 'bg-blue-600 text-white' : 'bg-white text-slate-700'}`} onClick={() => setView('team')}>Team</button>
                  <button className={`px-3 py-1 text-sm font-medium ${view === 'audit' ? 'bg-blue-600 text-white' : 'bg-white text-slate-700'}`} onClick={() => setView('audit')}>Audit</button>
                </>
              )}
            </div>

            {view === 'calendar' && (
              <div className="inline-flex border rounded overflow-hidden">
                <button className={`px-3 py-1 text-sm ${calendarMode === 'month' ? 'bg-gray-900 text-white' : 'bg-white'}`} onClick={() => setCalendarMode('month')}>Month</button>
                <button className={`px-3 py-1 text-sm ${calendarMode === 'week' ? 'bg-gray-900 text-white' : 'bg-white'}`} onClick={() => setCalendarMode('week')}>Week</button>
              </div>
            )}
          </div>
        </div>

        {view === 'team' ? (
          <TeamManagement tenant={tenant} />
        ) : view === 'setup' ? (
          <ClinicSetup tenant={tenant} />
        ) : view === 'audit' ? (
          <AuditLogTable tenant={tenant} />
        ) : (
          <>
            {view === 'table' && (
              <div className="bg-white border rounded p-3 shadow-sm space-y-2">
                <div className="grid grid-cols-6 gap-2">
                  <input value={fPatient} onChange={(e)=>setFPatient(e.target.value)} className="border rounded p-2 text-sm" placeholder="Filter patient" />
                  <input value={fDoctor} onChange={(e)=>setFDoctor(e.target.value)} className="border rounded p-2 text-sm" placeholder="Filter doctor" />
                  <input value={fDate} onChange={(e)=>setFDate(e.target.value)} className="border rounded p-2 text-sm" placeholder="Filter date YYYY-MM-DD" />
                  <input value={fTime} onChange={(e)=>setFTime(e.target.value)} className="border rounded p-2 text-sm" placeholder="Filter time HH:MM" />
                  <input value={fReason} onChange={(e)=>setFReason(e.target.value)} className="border rounded p-2 text-sm" placeholder="Filter reason" />
                  <select value={fStatus} onChange={(e)=>setFStatus(e.target.value)} className="border rounded p-2 text-sm bg-white">
                    <option value="">All statuses</option>
                    <option value="pending">pending</option>
                    <option value="confirmed">confirmed</option>
                    <option value="cancelled">cancelled</option>
                    <option value="completed">completed</option>
                    <option value="no_show">no_show</option>
                  </select>
                </div>
                <div className="flex justify-end">
                  <button className="text-sm text-gray-600 hover:underline" onClick={clearFilters}>Clear filters</button>
                </div>
              </div>
            )}

            {loading ? (
              <p className="text-center py-6 text-gray-500">Loading appointments...</p>
            ) : view === 'table' ? (
              <AppointmentTable
                appointments={filtered}
                onEdit={canEditAppointments ? setEditing : undefined}
                onCancel={canCancelAppointments ? onCancel : undefined}
                onDelete={canDeleteAppointments ? onDelete : undefined}
                canEdit={canEditAppointments}
                canCancel={canCancelAppointments}
                canDelete={canDeleteAppointments}
              />
            ) : (
              <Calendar
                view={calendarMode}
                date={currentDate}
                onChangeDate={setCurrentDate}
                appointments={filtered}
                onAdd={canCreateAppointments ? onAddFromCalendar : undefined}
                onEdit={canEditAppointments ? setEditing : undefined}
                onCancel={canCancelAppointments ? onCancel : undefined}
                onDelete={canDeleteAppointments ? onDelete : undefined}
                canCreate={canCreateAppointments}
                canEdit={canEditAppointments}
                canCancel={canCancelAppointments}
                canDelete={canDeleteAppointments}
              />
            )}
          </>
        )}

        {editing && (
          <div className="fixed inset-0 bg-black/40 grid place-items-center p-4 z-50 overflow-y-auto">
            <div className="bg-white rounded-xl shadow-xl w-full max-w-lg p-5 space-y-4">
              <h3 className="text-lg font-bold text-gray-900">
                {editing.id ? `Edit Appointment #${editing.id}` : 'Create Appointment'}
              </h3>

              {modalError && (
                <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700 space-y-2">
                  <p className="font-semibold">{modalError}</p>
                  {conflictAlternatives.length > 0 && (
                    <div className="mt-2 pt-2 border-t border-red-200">
                      <p className="text-xs font-semibold text-gray-700 mb-1.5">
                        Available alternative slots:
                      </p>
                      <div className="flex flex-wrap gap-1.5">
                        {conflictAlternatives.map((alt) => (
                          <button
                            type="button"
                            key={alt}
                            onClick={() => {
                              setEditing((prev) => ({ ...prev, time: alt }))
                              setConflictAlternatives([])
                              setModalError('')
                            }}
                            className="px-2.5 py-1 text-xs font-bold bg-white text-blue-700 border border-blue-300 rounded shadow-sm hover:bg-blue-50 transition"
                          >
                            ⏱️ {alt}
                          </button>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )}

              <div className="grid grid-cols-2 gap-3">
                <L label="Patient Name *">
                  <input
                    required
                    className="border rounded p-2 w-full text-sm focus:ring focus:outline-none"
                    value={editing.patient_name || ''}
                    onChange={(e) => setEditing({ ...editing, patient_name: e.target.value })}
                    placeholder="e.g. John Doe"
                  />
                </L>

                <L label="Doctor *">
                  <select
                    required
                    className="border rounded p-2 w-full text-sm bg-white focus:ring focus:outline-none"
                    value={editing.doctor || ''}
                    onChange={(e) => setEditing({ ...editing, doctor: e.target.value, time: '' })}
                  >
                    <option value="">Select a doctor...</option>
                    {activeDoctors.map((doc) => (
                      <option key={doc.id} value={doc.name}>
                        {doc.name} {doc.specialty ? `(${doc.specialty})` : ''}
                      </option>
                    ))}
                    {editing.doctor && !activeDoctors.some((d) => d.name === editing.doctor) && (
                      <option value={editing.doctor}>{editing.doctor}</option>
                    )}
                  </select>
                </L>

                <L label="Date *">
                  <input
                    type="date"
                    required
                    min={todayInNairobi}
                    className="border rounded p-2 w-full text-sm focus:ring focus:outline-none"
                    value={editing.date || ''}
                    onChange={(e) => setEditing({ ...editing, date: e.target.value, time: '' })}
                  />
                  <span className="text-[10px] text-gray-500">Clinic Timezone: Africa/Nairobi</span>
                </L>

                <L label="Time *">
                  {editing.doctor && editing.date && availableSlots.length > 0 ? (
                    <select
                      className="border rounded p-2 w-full text-sm bg-white focus:ring focus:outline-none"
                      value={editing.time || ''}
                      onChange={(e) => setEditing({ ...editing, time: e.target.value })}
                    >
                      <option value="">Select an available slot...</option>
                      {availableSlots.map((slot) => (
                        <option key={slot} value={slot}>
                          {slot}
                        </option>
                      ))}
                      {editing.time && !availableSlots.includes(editing.time) && (
                        <option value={editing.time}>{editing.time} (Current)</option>
                      )}
                    </select>
                  ) : (
                    <input
                      type="text"
                      required
                      placeholder="HH:MM (e.g. 10:00)"
                      className="border rounded p-2 w-full text-sm focus:ring focus:outline-none"
                      value={editing.time || ''}
                      onChange={(e) => setEditing({ ...editing, time: e.target.value })}
                    />
                  )}
                  {loadingSlots && (
                    <span className="text-[10px] text-blue-600 animate-pulse">Loading slots...</span>
                  )}
                  {!loadingSlots && editing.doctor && editing.date && availableSlots.length === 0 && (
                    <span className="text-[10px] text-amber-600">No free slots on this date</span>
                  )}
                </L>

                <L label="Reason" className="col-span-2">
                  <input
                    className="border rounded p-2 w-full text-sm focus:ring focus:outline-none"
                    value={editing.reason || ''}
                    onChange={(e) => setEditing({ ...editing, reason: e.target.value })}
                    placeholder="e.g. General consultation"
                  />
                </L>

                <L label="Status" className="col-span-2">
                  <select
                    className="border rounded p-2 w-full text-sm bg-white focus:ring focus:outline-none"
                    value={editing.status || 'pending'}
                    onChange={(e) => setEditing({ ...editing, status: e.target.value })}
                  >
                    <option value="pending">pending</option>
                    <option value="confirmed">confirmed</option>
                    <option value="cancelled">cancelled</option>
                    <option value="completed">completed</option>
                    <option value="no_show">no_show</option>
                  </select>
                </L>
              </div>

              <div className="flex justify-end gap-2 pt-3 border-t">
                <button
                  type="button"
                  className="px-4 py-2 rounded-lg border text-sm text-gray-700 hover:bg-gray-50 transition"
                  onClick={() => {
                    setEditing(null)
                    setModalError('')
                    setConflictAlternatives([])
                  }}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  className="px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium shadow-sm transition"
                  onClick={onSave}
                >
                  {editing.id ? 'Save Changes' : 'Create Appointment'}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function L({ label, children, className = '' }) {
  return (
    <label className={`flex flex-col gap-1 ${className}`}>
      <span className="text-xs font-semibold text-gray-600">{label}</span>
      {children}
    </label>
  )
}
