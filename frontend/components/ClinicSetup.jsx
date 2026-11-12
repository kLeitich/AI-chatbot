"use client"
import { useEffect, useState, useCallback } from 'react'
import api from '../lib/api'
import { usePermissions } from '../context/PermissionContext'

export default function ClinicSetup({ tenant = 'default' }) {
  const { can } = usePermissions()

  const canManageDoctors = can('doctors.manage')
  const canManageReasons = can('reasons.manage')
  const canManageSettings = can('settings.manage')

  // --- Doctors State ---
  const [doctors, setDoctors] = useState([])
  const [loadingDoctors, setLoadingDoctors] = useState(false)
  const [editingDoctor, setEditingDoctor] = useState(null)
  const [doctorForm, setDoctorForm] = useState({
    name: '',
    specialty: '',
    bio: '',
    work_days: 'Mon,Tue,Wed,Thu,Fri',
    work_start: '09:00',
    work_end: '17:00',
    slot_minutes: 30,
  })
  const [doctorMsg, setDoctorMsg] = useState({ error: '', success: '' })

  // --- Reasons State ---
  const [reasons, setReasons] = useState([])
  const [loadingReasons, setLoadingReasons] = useState(false)
  const [newReasonLabel, setNewReasonLabel] = useState('')
  const [reasonMsg, setReasonMsg] = useState({ error: '', success: '' })

  // --- Settings State ---
  const [settings, setSettings] = useState({
    emergency_number: '',
    emergency_message: '',
  })
  const [loadingSettings, setLoadingSettings] = useState(false)
  const [settingsMsg, setSettingsMsg] = useState({ error: '', success: '' })

  // --- Fetchers ---
  const fetchDoctors = useCallback(async () => {
    if (!canManageDoctors) return
    setLoadingDoctors(true)
    try {
      const res = await api.get(`/t/${tenant}/admin/doctors`)
      setDoctors(res.data || [])
    } catch (err) {
      setDoctorMsg({ error: err?.response?.data?.message || 'Failed to load doctors', success: '' })
    } finally {
      setLoadingDoctors(false)
    }
  }, [tenant, canManageDoctors])

  const fetchReasons = useCallback(async () => {
    if (!canManageReasons) return
    setLoadingReasons(true)
    try {
      const res = await api.get(`/t/${tenant}/admin/reasons`)
      setReasons(res.data || [])
    } catch (err) {
      setReasonMsg({ error: err?.response?.data?.message || 'Failed to load reasons', success: '' })
    } finally {
      setLoadingReasons(false)
    }
  }, [tenant, canManageReasons])

  const fetchSettings = useCallback(async () => {
    if (!canManageSettings) return
    setLoadingSettings(true)
    try {
      const res = await api.get(`/t/${tenant}/admin/settings`)
      setSettings({
        emergency_number: res.data?.emergency_number || '',
        emergency_message: res.data?.emergency_message || '',
      })
    } catch (err) {
      setSettingsMsg({ error: err?.response?.data?.message || 'Failed to load settings', success: '' })
    } finally {
      setLoadingSettings(false)
    }
  }, [tenant, canManageSettings])

  useEffect(() => {
    fetchDoctors()
    fetchReasons()
    fetchSettings()
  }, [fetchDoctors, fetchReasons, fetchSettings])

  // --- Doctor Handlers ---
  const handleSaveDoctor = async (e) => {
    e.preventDefault()
    setDoctorMsg({ error: '', success: '' })

    if (!doctorForm.name.trim()) {
      setDoctorMsg({ error: 'Doctor name is required', success: '' })
      return
    }

    try {
      if (editingDoctor) {
        await api.patch(`/t/${tenant}/admin/doctors/${editingDoctor.id}`, {
          name: doctorForm.name,
          specialty: doctorForm.specialty,
          bio: doctorForm.bio,
          work_days: doctorForm.work_days,
          work_start: doctorForm.work_start,
          work_end: doctorForm.work_end,
          slot_minutes: Number(doctorForm.slot_minutes),
        })
        setDoctorMsg({ error: '', success: 'Doctor updated successfully!' })
      } else {
        await api.post(`/t/${tenant}/admin/doctors`, {
          ...doctorForm,
          slot_minutes: Number(doctorForm.slot_minutes),
        })
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
      })
      fetchDoctors()
    } catch (err) {
      setDoctorMsg({ error: err?.response?.data?.message || 'Failed to save doctor', success: '' })
    }
  }

  const handleToggleDoctorActive = async (doc) => {
    try {
      await api.patch(`/t/${tenant}/admin/doctors/${doc.id}`, {
        active: !doc.active,
      })
      fetchDoctors()
    } catch (err) {
      alert(err?.response?.data?.message || 'Failed to toggle status')
    }
  }

  const startEditDoctor = (doc) => {
    setEditingDoctor(doc)
    setDoctorForm({
      name: doc.name || '',
      specialty: doc.specialty || '',
      bio: doc.bio || '',
      work_days: doc.work_days || 'Mon,Tue,Wed,Thu,Fri',
      work_start: doc.work_start || '09:00',
      work_end: doc.work_end || '17:00',
      slot_minutes: doc.slot_minutes || 30,
    })
  }

  // --- Reason Handlers ---
  const handleAddReason = async (e) => {
    e.preventDefault()
    if (!newReasonLabel.trim()) return
    setReasonMsg({ error: '', success: '' })
    try {
      await api.post(`/t/${tenant}/admin/reasons`, { label: newReasonLabel.trim() })
      setNewReasonLabel('')
      setReasonMsg({ error: '', success: 'Reason added successfully!' })
      fetchReasons()
    } catch (err) {
      setReasonMsg({ error: err?.response?.data?.message || 'Failed to add reason', success: '' })
    }
  }

  const handleToggleReasonActive = async (reason) => {
    try {
      await api.patch(`/t/${tenant}/admin/reasons/${reason.id}`, {
        active: !reason.active,
      })
      fetchReasons()
    } catch (err) {
      alert(err?.response?.data?.message || 'Failed to toggle status')
    }
  }

  // --- Settings Handlers ---
  const handleSaveSettings = async (e) => {
    e.preventDefault()
    setSettingsMsg({ error: '', success: '' })
    try {
      await api.patch(`/t/${tenant}/admin/settings`, {
        emergency_number: settings.emergency_number,
        emergency_message: settings.emergency_message,
      })
      setSettingsMsg({ error: '', success: 'Emergency settings saved successfully!' })
    } catch (err) {
      setSettingsMsg({ error: err?.response?.data?.message || 'Failed to save settings', success: '' })
    }
  }

  return (
    <div className="space-y-8">
      {/* SECTION 1: DOCTORS */}
      {canManageDoctors && (
        <section className="bg-white border rounded-xl p-5 shadow-sm space-y-4">
          <div className="flex items-center justify-between border-b pb-3">
            <div>
              <h2 className="text-lg font-bold text-gray-900">Clinic Doctors</h2>
              <p className="text-xs text-gray-500">Manage doctor profiles, working hours, and active status.</p>
            </div>
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
                className="text-xs text-blue-600 hover:underline"
              >
                + Switch to Add New Doctor
              </button>
            )}
          </div>

          {doctorMsg.error && <div className="p-3 bg-red-50 text-red-700 text-sm rounded-lg">{doctorMsg.error}</div>}
          {doctorMsg.success && <div className="p-3 bg-green-50 text-green-700 text-sm rounded-lg">{doctorMsg.success}</div>}

          {/* Form */}
          <form onSubmit={handleSaveDoctor} className="bg-gray-50 p-4 rounded-lg space-y-3 border">
            <h3 className="text-sm font-semibold text-gray-800">
              {editingDoctor ? `Edit ${editingDoctor.name}` : 'Add New Doctor'}
            </h3>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Doctor Name *</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Dr. Jane Smith"
                  className="w-full border rounded p-2 text-sm bg-white"
                  value={doctorForm.name}
                  onChange={(e) => setDoctorForm({ ...doctorForm, name: e.target.value })}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Specialty</label>
                <input
                  type="text"
                  placeholder="e.g. Cardiology"
                  className="w-full border rounded p-2 text-sm bg-white"
                  value={doctorForm.specialty}
                  onChange={(e) => setDoctorForm({ ...doctorForm, specialty: e.target.value })}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Work Days</label>
                <input
                  type="text"
                  placeholder="Mon,Tue,Wed,Thu,Fri"
                  className="w-full border rounded p-2 text-sm bg-white"
                  value={doctorForm.work_days}
                  onChange={(e) => setDoctorForm({ ...doctorForm, work_days: e.target.value })}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Work Start (HH:MM)</label>
                <input
                  type="text"
                  required
                  placeholder="09:00"
                  className="w-full border rounded p-2 text-sm bg-white"
                  value={doctorForm.work_start}
                  onChange={(e) => setDoctorForm({ ...doctorForm, work_start: e.target.value })}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Work End (HH:MM)</label>
                <input
                  type="text"
                  required
                  placeholder="17:00"
                  className="w-full border rounded p-2 text-sm bg-white"
                  value={doctorForm.work_end}
                  onChange={(e) => setDoctorForm({ ...doctorForm, work_end: e.target.value })}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Slot Duration (10-120 min)</label>
                <input
                  type="number"
                  min={10}
                  max={120}
                  className="w-full border rounded p-2 text-sm bg-white"
                  value={doctorForm.slot_minutes}
                  onChange={(e) => setDoctorForm({ ...doctorForm, slot_minutes: e.target.value })}
                />
              </div>
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">Bio</label>
              <textarea
                rows={2}
                placeholder="Doctor experience and background..."
                className="w-full border rounded p-2 text-sm bg-white"
                value={doctorForm.bio}
                onChange={(e) => setDoctorForm({ ...doctorForm, bio: e.target.value })}
              />
            </div>
            <button
              type="submit"
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition"
            >
              {editingDoctor ? 'Update Doctor' : 'Add Doctor'}
            </button>
          </form>

          {/* List */}
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="bg-gray-100 text-left text-xs uppercase text-gray-600">
                <tr>
                  <th className="p-2">Name</th>
                  <th className="p-2">Specialty</th>
                  <th className="p-2">Hours</th>
                  <th className="p-2">Slot</th>
                  <th className="p-2">Status</th>
                  <th className="p-2">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {loadingDoctors ? (
                  <tr>
                    <td colSpan={6} className="p-4 text-center text-gray-500">Loading doctors...</td>
                  </tr>
                ) : doctors.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="p-4 text-center text-gray-500">No doctors configured.</td>
                  </tr>
                ) : (
                  doctors.map((doc) => (
                    <tr key={doc.id} className={doc.active === false ? 'bg-gray-50 opacity-75' : ''}>
                      <td className="p-2 font-medium">{doc.name}</td>
                      <td className="p-2 text-gray-600">{doc.specialty || 'General'}</td>
                      <td className="p-2 text-xs text-gray-500">{doc.work_start} - {doc.work_end} ({doc.work_days})</td>
                      <td className="p-2 text-xs">{doc.slot_minutes} min</td>
                      <td className="p-2">
                        <span className={`px-2 py-0.5 rounded text-xs font-bold ${
                          doc.active !== false ? 'bg-green-100 text-green-800' : 'bg-gray-200 text-gray-700'
                        }`}>
                          {doc.active !== false ? 'Active' : 'Inactive'}
                        </span>
                      </td>
                      <td className="p-2">
                        <div className="flex gap-2">
                          <button
                            onClick={() => startEditDoctor(doc)}
                            className="px-2 py-1 text-xs border rounded hover:bg-gray-50"
                          >
                            Edit
                          </button>
                          <button
                            onClick={() => handleToggleDoctorActive(doc)}
                            className={`px-2 py-1 text-xs rounded text-white ${
                              doc.active !== false ? 'bg-amber-600 hover:bg-amber-700' : 'bg-emerald-600 hover:bg-emerald-700'
                            }`}
                          >
                            {doc.active !== false ? 'Deactivate' : 'Activate'}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* SECTION 2: VISIT REASONS */}
      {canManageReasons && (
        <section className="bg-white border rounded-xl p-5 shadow-sm space-y-4">
          <div className="border-b pb-3">
            <h2 className="text-lg font-bold text-gray-900">Visit Reasons</h2>
            <p className="text-xs text-gray-500">Configure standard visit reasons available for appointment booking.</p>
          </div>

          {reasonMsg.error && <div className="p-3 bg-red-50 text-red-700 text-sm rounded-lg">{reasonMsg.error}</div>}
          {reasonMsg.success && <div className="p-3 bg-green-50 text-green-700 text-sm rounded-lg">{reasonMsg.success}</div>}

          {/* Add Form */}
          <form onSubmit={handleAddReason} className="flex gap-2">
            <input
              type="text"
              required
              placeholder="e.g. Dental Checkup"
              className="flex-1 border rounded p-2 text-sm bg-white"
              value={newReasonLabel}
              onChange={(e) => setNewReasonLabel(e.target.value)}
            />
            <button
              type="submit"
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition"
            >
              Add Reason
            </button>
          </form>

          {/* List */}
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="bg-gray-100 text-left text-xs uppercase text-gray-600">
                <tr>
                  <th className="p-2">Reason Label</th>
                  <th className="p-2">Status</th>
                  <th className="p-2">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {loadingReasons ? (
                  <tr>
                    <td colSpan={3} className="p-4 text-center text-gray-500">Loading reasons...</td>
                  </tr>
                ) : reasons.length === 0 ? (
                  <tr>
                    <td colSpan={3} className="p-4 text-center text-gray-500">No reasons configured.</td>
                  </tr>
                ) : (
                  reasons.map((r) => (
                    <tr key={r.id} className={r.active === false ? 'bg-gray-50 opacity-75' : ''}>
                      <td className="p-2 font-medium">{r.label}</td>
                      <td className="p-2">
                        <span className={`px-2 py-0.5 rounded text-xs font-bold ${
                          r.active !== false ? 'bg-green-100 text-green-800' : 'bg-gray-200 text-gray-700'
                        }`}>
                          {r.active !== false ? 'Active' : 'Inactive'}
                        </span>
                      </td>
                      <td className="p-2">
                        <button
                          onClick={() => handleToggleReasonActive(r)}
                          className={`px-2 py-1 text-xs rounded text-white ${
                            r.active !== false ? 'bg-amber-600 hover:bg-amber-700' : 'bg-emerald-600 hover:bg-emerald-700'
                          }`}
                        >
                          {r.active !== false ? 'Deactivate' : 'Activate'}
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* SECTION 3: EMERGENCY SETTINGS */}
      {canManageSettings && (
        <section className="bg-white border rounded-xl p-5 shadow-sm space-y-4">
          <div className="border-b pb-3">
            <h2 className="text-lg font-bold text-gray-900">Emergency Settings</h2>
            <p className="text-xs text-gray-500">Configure emergency phone contact and instructions shown to patients during urgent chat emergencies.</p>
          </div>

          {settingsMsg.error && <div className="p-3 bg-red-50 text-red-700 text-sm rounded-lg">{settingsMsg.error}</div>}
          {settingsMsg.success && <div className="p-3 bg-green-50 text-green-700 text-sm rounded-lg">{settingsMsg.success}</div>}

          <form onSubmit={handleSaveSettings} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Emergency Phone Number (Digits, +, spaces, max 20)
              </label>
              <input
                type="text"
                maxLength={20}
                placeholder="e.g. +254 700 000 000 or 999"
                className="w-full border rounded p-2 text-sm bg-white"
                value={settings.emergency_number}
                onChange={(e) => setSettings({ ...settings, emergency_number: e.target.value })}
              />
              <span className="text-[10px] text-gray-500">Defaults to 999 if left empty.</span>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Custom Emergency Message / Instructions (Max 500 characters)
              </label>
              <textarea
                rows={3}
                maxLength={500}
                placeholder="If you are experiencing severe symptoms, please proceed to the nearest emergency department immediately."
                className="w-full border rounded p-2 text-sm bg-white"
                value={settings.emergency_message}
                onChange={(e) => setSettings({ ...settings, emergency_message: e.target.value })}
              />
            </div>

            {/* Live Preview */}
            <div className="p-4 bg-red-50 border-2 border-red-200 rounded-xl space-y-2">
              <h4 className="text-xs font-bold text-red-900 uppercase tracking-wide">Live Chat Emergency Card Preview</h4>
              <p className="text-sm text-red-900 whitespace-pre-line font-medium">
                {settings.emergency_message?.trim() || "If you are experiencing severe chest pain, shortness of breath, or bleeding, please call emergency services immediately."}
              </p>
              <div className="flex gap-2 pt-2">
                <span className="px-3 py-1 bg-white text-red-800 text-xs font-bold rounded shadow-sm border border-red-300">
                  📞 Call {settings.emergency_number?.trim() || '999'}
                </span>
                <span className="px-3 py-1 bg-white text-red-800 text-xs font-bold rounded shadow-sm border border-red-300">
                  📞 Call 999
                </span>
                <span className="px-3 py-1 bg-red-800 text-white text-xs font-bold rounded shadow-sm">
                  📞 Call 112
                </span>
              </div>
            </div>

            <button
              type="submit"
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition"
            >
              Save Emergency Settings
            </button>
          </form>
        </section>
      )}
    </div>
  )
}

