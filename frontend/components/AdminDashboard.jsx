"use client"
import { useEffect, useMemo, useState } from 'react'
import { useRouter } from 'next/navigation'
import api from '../lib/api'
import AppointmentTable from './AppointmentTable'
import Calendar from './Calendar'

export default function AdminDashboard() {
  const [appointments, setAppointments] = useState([])
  const [editing, setEditing] = useState(null)
  const [loading, setLoading] = useState(true)
  const [view, setView] = useState('table')
  const [calendarMode, setCalendarMode] = useState('month')
  const [currentDate, setCurrentDate] = useState(new Date())
  const router = useRouter()

  // Filters
  const [fPatient, setFPatient] = useState('')
  const [fDoctor, setFDoctor] = useState('')
  const [fDate, setFDate] = useState('')
  const [fTime, setFTime] = useState('')
  const [fReason, setFReason] = useState('')
  const [fStatus, setFStatus] = useState('')

  const load = async () => {
    setLoading(true)
    try {
      const res = await api.get('/admin/appointments')
      setAppointments(res.data || [])
    } catch (e) {
      if (e?.response?.status === 401) {
        localStorage.removeItem('token')
        router.replace('/admin/login')
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

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

  const onDelete = async (a) => {
    if (!confirm(`Delete appointment #${a.id}?`)) return
    await api.delete(`/admin/appointments/${a.id}`)
    load()
  }

  const onSave = async () => {
    const { id, ...rest } = editing
    if (id) {
      await api.put(`/admin/appointments/${id}`, rest)
    } else {
      await api.post('/admin/appointments', rest)
    }
    setEditing(null)
    await load() // Always refresh for modal adds/edits
  }

  const onAddFromCalendar = (draft) => { setEditing({ ...draft }) }
  const onAddFromTable = () => {
    setEditing({ patient_name: '', doctor: '', date: '', time: '', reason: '', status: 'pending' })
  }

  return (
    <div className="max-w-5xl mx-auto p-4 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Admin Dashboard</h1>
        <div className="flex items-center gap-3">
          <button className="px-3 py-1 rounded bg-blue-600 text-white" onClick={onAddFromTable}>New</button>
          <div className="inline-flex border rounded overflow-hidden">
            <button className={`px-3 py-1 text-sm ${view === 'table' ? 'bg-blue-600 text-white' : 'bg-white'}`} onClick={() => setView('table')}>Table</button>
            <button className={`px-3 py-1 text-sm ${view === 'calendar' ? 'bg-blue-600 text-white' : 'bg-white'}`} onClick={() => setView('calendar')}>Calendar</button>
          </div>
          {view === 'calendar' && (
            <div className="inline-flex border rounded overflow-hidden">
              <button className={`px-3 py-1 text-sm ${calendarMode === 'month' ? 'bg-gray-900 text-white' : 'bg-white'}`} onClick={() => setCalendarMode('month')}>Month</button>
              <button className={`px-3 py-1 text-sm ${calendarMode === 'week' ? 'bg-gray-900 text-white' : 'bg-white'}`} onClick={() => setCalendarMode('week')}>Week</button>
            </div>
          )}
          <button className="text-sm text-red-600" onClick={() => { localStorage.removeItem('token'); router.replace('/admin/login') }}>Log out</button>
        </div>
      </div>

      {view === 'table' && (
        <div className="bg-white border rounded p-3 shadow space-y-2">
          <div className="grid grid-cols-6 gap-2">
            <input value={fPatient} onChange={(e)=>setFPatient(e.target.value)} className="border rounded p-2" placeholder="Filter patient" />
            <input value={fDoctor} onChange={(e)=>setFDoctor(e.target.value)} className="border rounded p-2" placeholder="Filter doctor" />
            <input value={fDate} onChange={(e)=>setFDate(e.target.value)} className="border rounded p-2" placeholder="Filter date YYYY-MM-DD" />
            <input value={fTime} onChange={(e)=>setFTime(e.target.value)} className="border rounded p-2" placeholder="Filter time HH:MM" />
            <input value={fReason} onChange={(e)=>setFReason(e.target.value)} className="border rounded p-2" placeholder="Filter reason" />
            <select value={fStatus} onChange={(e)=>setFStatus(e.target.value)} className="border rounded p-2">
              <option value="">All statuses</option>
              <option value="pending">pending</option>
              <option value="confirmed">confirmed</option>
              <option value="cancelled">cancelled</option>
            </select>
          </div>
          <div className="flex justify-end">
            <button className="text-sm text-gray-600 hover:underline" onClick={clearFilters}>Clear filters</button>
          </div>
        </div>
      )}

      {loading ? (
        <p>Loading...</p>
      ) : view === 'table' ? (
        <AppointmentTable appointments={filtered} onEdit={setEditing} onDelete={onDelete} />
      ) : (
        <Calendar view={calendarMode} date={currentDate} onChangeDate={setCurrentDate} appointments={filtered} onAdd={onAddFromCalendar} onEdit={setEditing} onDelete={onDelete} />
      )}

      {editing && (
        <div className="fixed inset-0 bg-black/40 grid place-items-center p-4">
          <div className="bg-white rounded shadow w-full max-w-lg p-4 space-y-3">
            <h3 className="text-lg font-medium">{editing.id ? `Edit Appointment #${editing.id}` : 'Create Appointment'}</h3>
            <div className="grid grid-cols-2 gap-3">
              <L label="Patient"><input className="border rounded p-2 w-full" value={editing.patient_name || ''} onChange={(e) => setEditing({ ...editing, patient_name: e.target.value })} /></L>
              <L label="Doctor"><input className="border rounded p-2 w-full" value={editing.doctor || ''} onChange={(e) => setEditing({ ...editing, doctor: e.target.value })} /></L>
              <L label="Date"><input className="border rounded p-2 w-full" value={editing.date || ''} onChange={(e) => setEditing({ ...editing, date: e.target.value })} placeholder="YYYY-MM-DD" /></L>
              <L label="Time"><input className="border rounded p-2 w-full" value={editing.time || ''} onChange={(e) => setEditing({ ...editing, time: e.target.value })} placeholder="HH:MM" /></L>
              <L label="Reason" className="col-span-2"><input className="border rounded p-2 w-full" value={editing.reason || ''} onChange={(e) => setEditing({ ...editing, reason: e.target.value })} /></L>
              <L label="Status" className="col-span-2">
                <select className="border rounded p-2 w-full" value={editing.status || 'pending'} onChange={(e) => setEditing({ ...editing, status: e.target.value })}>
                  <option value="pending">pending</option>
                  <option value="confirmed">confirmed</option>
                  <option value="cancelled">cancelled</option>
                </select>
              </L>
            </div>
            <div className="flex justify-end gap-2">
              <button className="px-4 py-2 rounded border" onClick={() => setEditing(null)}>Cancel</button>
              <button className="px-4 py-2 rounded bg-blue-600 text-white" onClick={onSave}>{editing.id ? 'Save' : 'Create'}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function L({ label, children, className = '' }) {
  return (
    <label className={`flex flex-col gap-1 ${className}`}>
      <span className="text-sm text-gray-600">{label}</span>
      {children}
    </label>
  )
}
