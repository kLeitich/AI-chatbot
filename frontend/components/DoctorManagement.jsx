"use client"
import React, { useEffect, useState, useCallback } from 'react'
import api from '../lib/api'
import { usePermissions } from '../context/PermissionContext'

export default function DoctorManagement({ tenant = 'default' }) {
  const { can } = usePermissions()

  const canManageDoctors = can('doctors.manage')
  const canViewDoctors = can('doctors.view') || canManageDoctors

  const [doctors, setDoctors] = useState([])
  const [loading, setLoading] = useState(false)
  const [editingDoctor, setEditingDoctor] = useState(null)
  const [doctorForm, setDoctorForm] = useState({
    name: '',
    specialty: '',
    bio: '',
    work_days: 'Mon,Tue,Wed,Thu,Fri',
    work_start: '09:00',
    work_end: '17:00',
    slot_minutes: 30,
    google_calendar_id: '',
  })
  const [doctorMsg, setDoctorMsg] = useState({ error: '', success: '' })

  const fetchDoctors = useCallback(async () => {
    if (!canViewDoctors) return
    setLoading(true)
    try {
      const res = await api.get(`/t/${tenant}/admin/doctors`)
      setDoctors(res.data || [])
    } catch (err) {
      setDoctorMsg({ error: err?.response?.data?.message || 'Failed to load doctors', success: '' })
    } finally {
      setLoading(false)
    }
  }, [tenant, canViewDoctors])

  useEffect(() => {
    fetchDoctors()
  }, [fetchDoctors])

  const handleSaveDoctor = async (e) => {
    e.preventDefault()
    if (!canManageDoctors) return
    setDoctorMsg({ error: '', success: '' })

    if (!doctorForm.name.trim()) {
      setDoctorMsg({ error: 'Doctor name is required', success: '' })
      return
    }

    try {
      if (editingDoctor) {
        await api.patch(`/t/${tenant}/admin/doctors/${editingDoctor.id}`, doctorForm)
        setDoctorMsg({ error: '', success: 'Doctor updated successfully!' })
      } else {
        await api.post(`/t/${tenant}/admin/doctors`, doctorForm)
        setDoctorMsg({ error: '', success: 'Doctor added successfully!' })
      }

      setEditingDoctor(null)
      setDoctorForm({
        name: '',
        specialty: '',
        bio: '',
        work_days: 'Mon,Tue,Wed,Thu,Fri',
        work_start: '09:00',
        work_end: '17:00',
        slot_minutes: 30,
        google_calendar_id: '',
      })
      fetchDoctors()
    } catch (err) {
      setDoctorMsg({ error: err?.response?.data?.message || 'Failed to save doctor', success: '' })
    }
  }

  const handleEditClick = (doc) => {
    if (!canManageDoctors) return
    setEditingDoctor(doc)
    setDoctorForm({
      name: doc.name || '',
      specialty: doc.specialty || '',
      bio: doc.bio || '',
      work_days: doc.work_days || 'Mon,Tue,Wed,Thu,Fri',
      work_start: doc.work_start || '09:00',
      work_end: doc.work_end || '17:00',
      slot_minutes: doc.slot_minutes || 30,
      google_calendar_id: doc.google_calendar_id || '',
    })
    setDoctorMsg({ error: '', success: '' })
  }

  const handleToggleActive = async (doc) => {
    if (!canManageDoctors) return
    const action = doc.active ? 'deactivate' : 'activate'
    if (!window.confirm(`Are you sure you want to ${action} ${doc.name}?`)) return

    try {
      await api.patch(`/t/${tenant}/admin/doctors/${doc.id}`, {
        active: !doc.active,
      })
      fetchDoctors()
    } catch (err) {
      alert(err?.response?.data?.message || 'Failed to update doctor status')
    }
  }

  if (!canViewDoctors) {
    return (
      <div className="bg-amber-50 border border-amber-200 rounded-xl p-6 text-amber-900 text-center space-y-2">
        <span className="text-2xl" aria-hidden="true">🔒</span>
        <h3 className="font-bold text-sm">Access Restricted</h3>
        <p className="text-xs text-amber-700">
          You lack the &quot;doctors.view&quot; or &quot;doctors.manage&quot; permission to view the clinic doctors directory.
        </p>
      </div>
    )
  }

  return (
    <section className="bg-white border rounded-xl p-5 shadow-sm space-y-6" data-testid="doctor-management">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b pb-4">
        <div>
          <h2 className="text-xl font-bold text-slate-900">Clinic Doctors</h2>
          <p className="text-xs text-slate-500">
            Manage physician profiles, working schedules, consultation slot lengths, and availability.
          </p>
        </div>
        {!canManageDoctors && (
          <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-slate-100 text-slate-600 border">
            Read-only (requires doctors.manage to edit)
          </span>
        )}
        {canManageDoctors && editingDoctor && (
          <button
            type="button"
            onClick={() => {
              setEditingDoctor(null)
              setDoctorForm({
                name: '',
                specialty: '',
                bio: '',
                work_days: 'Mon,Tue,Wed,Thu,Fri',
                work_start: '09:00',
                work_end: '17:00',
                slot_minutes: 30,
              })
            }}
            className="text-xs text-blue-600 hover:underline font-medium"
          >
            + Switch to Add New Doctor
          </button>
        )}
      </div>

      {doctorMsg.error && (
        <div className="p-3 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg">
          {doctorMsg.error}
        </div>
      )}
      {doctorMsg.success && (
        <div className="p-3 bg-green-50 border border-green-200 text-green-700 text-sm rounded-lg">
          {doctorMsg.success}
        </div>
      )}

      {/* Add / Edit Form (gated by doctors.manage) */}
      {canManageDoctors && (
        <form onSubmit={handleSaveDoctor} className="bg-slate-50 p-4 rounded-xl space-y-4 border">
          <h3 className="text-sm font-bold text-slate-800">
            {editingDoctor ? `Edit ${editingDoctor.name}` : 'Add New Doctor'}
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">Doctor Name *</label>
              <input
                type="text"
                required
                placeholder="e.g. Dr. Jane Smith"
                className="w-full border rounded-lg p-2 text-sm bg-white"
                value={doctorForm.name}
                onChange={(e) => setDoctorForm({ ...doctorForm, name: e.target.value })}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">Specialty</label>
              <input
                type="text"
                placeholder="e.g. Cardiology"
                className="w-full border rounded-lg p-2 text-sm bg-white"
                value={doctorForm.specialty}
                onChange={(e) => setDoctorForm({ ...doctorForm, specialty: e.target.value })}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">Slot Length (Minutes)</label>
              <input
                type="number"
                min={5}
                max={120}
                className="w-full border rounded-lg p-2 text-sm bg-white"
                value={doctorForm.slot_minutes}
                onChange={(e) => setDoctorForm({ ...doctorForm, slot_minutes: Number(e.target.value) })}
              />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">Work Days (comma-separated)</label>
              <input
                type="text"
                placeholder="Mon,Tue,Wed,Thu,Fri"
                className="w-full border rounded-lg p-2 text-sm bg-white"
                value={doctorForm.work_days}
                onChange={(e) => setDoctorForm({ ...doctorForm, work_days: e.target.value })}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">Shift Start Time (HH:MM)</label>
              <input
                type="text"
                placeholder="09:00"
                className="w-full border rounded-lg p-2 text-sm bg-white"
                value={doctorForm.work_start}
                onChange={(e) => setDoctorForm({ ...doctorForm, work_start: e.target.value })}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">Shift End Time (HH:MM)</label>
              <input
                type="text"
                placeholder="17:00"
                className="w-full border rounded-lg p-2 text-sm bg-white"
                value={doctorForm.work_end}
                onChange={(e) => setDoctorForm({ ...doctorForm, work_end: e.target.value })}
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1">Bio / Profile Notes</label>
            <textarea
              rows={2}
              placeholder="e.g. Specialist in cardiovascular wellness and preventative medicine."
              className="w-full border rounded-lg p-2 text-sm bg-white"
              value={doctorForm.bio}
              onChange={(e) => setDoctorForm({ ...doctorForm, bio: e.target.value })}
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1">
              Google Calendar ID (Optional - e.g. dr.smith@clinic.com)
            </label>
            <input
              type="text"
              placeholder="primary or doctor's specific Google Calendar email"
              className="w-full border rounded-lg p-2 text-sm bg-white"
              value={doctorForm.google_calendar_id}
              onChange={(e) => setDoctorForm({ ...doctorForm, google_calendar_id: e.target.value })}
            />
            <span className="text-[10px] text-slate-500">
              Leave blank to sync to the clinic&apos;s default calendar.
            </span>
          </div>

          <div className="flex justify-end gap-2">
            {editingDoctor && (
              <button
                type="button"
                onClick={() => {
                  setEditingDoctor(null)
                  setDoctorForm({
                    name: '',
                    specialty: '',
                    bio: '',
                    work_days: 'Mon,Tue,Wed,Thu,Fri',
                    work_start: '09:00',
                    work_end: '17:00',
                    slot_minutes: 30,
                  })
                }}
                className="px-3 py-1.5 border rounded-lg text-xs font-medium text-slate-600 hover:bg-slate-100 transition"
              >
                Cancel
              </button>
            )}
            <button
              type="submit"
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-semibold shadow-sm transition"
            >
              {editingDoctor ? 'Update Doctor' : 'Add Doctor'}
            </button>
          </div>
        </form>
      )}

      {/* Doctor Cards / Table */}
      {loading ? (
        <div className="text-center py-8 text-slate-400 text-xs">Loading doctors directory...</div>
      ) : doctors.length === 0 ? (
        <div className="text-center py-8 text-slate-400 text-xs">No doctors configured yet.</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {doctors.map((doc) => (
            <div
              key={doc.id}
              className="border rounded-xl p-4 bg-white shadow-sm flex flex-col justify-between space-y-3"
            >
              <div>
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <h4 className="font-bold text-slate-900 text-sm">{doc.name}</h4>
                    <p className="text-xs text-blue-600 font-medium">{doc.specialty || 'General Practice'}</p>
                  </div>
                  <span
                    className={`px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider ${
                      doc.active ? 'bg-emerald-100 text-emerald-800' : 'bg-slate-100 text-slate-500 line-through'
                    }`}
                  >
                    {doc.active ? 'Active' : 'Inactive'}
                  </span>
                </div>

                {doc.bio && <p className="text-xs text-slate-500 mt-2 line-clamp-2">{doc.bio}</p>}

                <div className="mt-3 pt-2 border-t border-slate-100 text-[11px] text-slate-600 space-y-1">
                  <div>
                    <span className="font-medium text-slate-500">Days:</span> {doc.work_days || 'Mon-Fri'}
                  </div>
                  <div>
                    <span className="font-medium text-slate-500">Hours:</span> {doc.work_start} - {doc.work_end} ({doc.slot_minutes || 30}m slots)
                  </div>
                </div>
              </div>

              {canManageDoctors && (
                <div className="pt-2 border-t border-slate-100 flex items-center justify-end gap-2 text-xs">
                  <button
                    type="button"
                    onClick={() => handleEditClick(doc)}
                    className="text-blue-600 hover:text-blue-800 font-medium"
                  >
                    Edit
                  </button>
                  <button
                    type="button"
                    onClick={() => handleToggleActive(doc)}
                    className={`font-medium ${doc.active ? 'text-rose-600 hover:text-rose-800' : 'text-emerald-600 hover:text-emerald-800'}`}
                  >
                    {doc.active ? 'Deactivate' : 'Activate'}
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </section>
  )
}
