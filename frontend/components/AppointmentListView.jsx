"use client"
import React, { useMemo } from 'react'

function getNairobiDateStr(d = new Date()) {
  try {
    return new Intl.DateTimeFormat('en-CA', {
      timeZone: 'Africa/Nairobi',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).format(d)
  } catch {
    const year = d.getFullYear()
    const month = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    return `${year}-${month}-${day}`
  }
}

function formatDayHeader(dateStr, todayStr, tomorrowStr) {
  let prefix = ''
  if (dateStr === todayStr) {
    prefix = 'Today'
  } else if (dateStr === tomorrowStr) {
    prefix = 'Tomorrow'
  }

  let formatted = dateStr
  try {
    const [y, m, d] = dateStr.split('-').map(Number)
    const obj = new Date(y, m - 1, d)
    formatted = obj.toLocaleDateString('en-US', {
      weekday: 'long',
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    })
  } catch {
    formatted = dateStr
  }

  return prefix ? `${prefix} — ${formatted}` : formatted
}

const statusBadges = {
  confirmed: 'bg-emerald-100 text-emerald-800 border-emerald-300',
  pending: 'bg-amber-100 text-amber-800 border-amber-300',
  cancelled: 'bg-rose-100 text-rose-800 border-rose-300',
  completed: 'bg-blue-100 text-blue-800 border-blue-300',
  no_show: 'bg-purple-100 text-purple-800 border-purple-300',
}

export default function AppointmentListView({
  appointments = [],
  rawCount = 0,
  onEdit,
  onCancel,
  onDelete,
  canEdit = true,
  canCancel = false,
  canDelete = true,
}) {
  const { todayStr, tomorrowStr } = useMemo(() => {
    const now = new Date()
    const t = getNairobiDateStr(now)
    const tom = new Date(now.getTime() + 24 * 60 * 60 * 1000)
    const tomStr = getNairobiDateStr(tom)
    return { todayStr: t, tomorrowStr: tomStr }
  }, [])

  // Sort by date ascending, then time ascending
  const sorted = useMemo(() => {
    return [...appointments].sort((a, b) => {
      const dComp = String(a.date || '').localeCompare(String(b.date || ''))
      if (dComp !== 0) return dComp
      return String(a.time || '').localeCompare(String(b.time || ''))
    })
  }, [appointments])

  // Group by date
  const grouped = useMemo(() => {
    const groups = {}
    for (const ap of sorted) {
      const k = ap.date || 'Unspecified Date'
      if (!groups[k]) groups[k] = []
      groups[k].push(ap)
    }
    return groups
  }, [sorted])

  const dayKeys = Object.keys(grouped)

  if (appointments.length === 0) {
    return (
      <div className="bg-white border rounded-xl p-12 text-center text-slate-500 shadow-sm space-y-2">
        <div className="text-3xl" aria-hidden="true">📅</div>
        {rawCount === 0 ? (
          <>
            <h3 className="text-base font-bold text-slate-800">No appointments found</h3>
            <p className="text-xs text-slate-500 max-w-sm mx-auto">
              Use &quot;+ New Appointment&quot; above to schedule your first clinic appointment.
            </p>
          </>
        ) : (
          <>
            <h3 className="text-base font-bold text-slate-800">No appointments match active filters</h3>
            <p className="text-xs text-slate-500 max-w-sm mx-auto">
              Try adjusting or clearing your search filters to view more appointments.
            </p>
          </>
        )}
      </div>
    )
  }

  return (
    <div className="space-y-6" data-testid="appointment-list-view">
      {dayKeys.map((dateStr) => {
        const dayAps = grouped[dateStr]
        const headerTitle = formatDayHeader(dateStr, todayStr, tomorrowStr)

        return (
          <section key={dateStr} className="space-y-3">
            <div className="flex items-center gap-2 border-b border-slate-200 pb-2">
              <h3 className="text-sm font-bold text-slate-900 tracking-tight">
                {headerTitle}
              </h3>
              <span className="text-xs text-slate-400 font-medium">
                ({dayAps.length} {dayAps.length === 1 ? 'appointment' : 'appointments'})
              </span>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
              {dayAps.map((ap) => {
                const st = ap.status || 'pending'
                const badgeClass = statusBadges[st] || 'bg-slate-100 text-slate-700 border-slate-300'
                const showEdit = Boolean(onEdit && canEdit)
                const showCancel = Boolean(onCancel && canCancel && ap.status !== 'cancelled')
                const showDelete = Boolean(onDelete && canDelete)
                const hasActions = showEdit || showCancel || showDelete

                return (
                  <div
                    key={ap.id ?? `${ap.date}-${ap.time}-${ap.patient_name}`}
                    className="bg-white border rounded-xl p-4 shadow-sm hover:shadow transition flex flex-col justify-between space-y-3"
                    data-testid="appointment-card"
                  >
                    <div className="space-y-2">
                      <div className="flex items-center justify-between gap-2">
                        <span className="font-bold text-sm text-blue-600 bg-blue-50 px-2.5 py-1 rounded-lg">
                          🕒 {ap.time}
                        </span>
                        <div className="flex items-center gap-1.5 flex-wrap">
                          {ap.source && (
                            <span
                              className="px-2 py-0.5 rounded-full text-[10px] font-medium bg-slate-100 text-slate-600 border border-slate-200"
                              title="Booking Source"
                            >
                              {ap.source}
                            </span>
                          )}
                          <span
                            className={`px-2.5 py-0.5 rounded-full text-[11px] font-bold uppercase tracking-wider border ${badgeClass}`}
                          >
                            {st.replace('_', ' ')}
                          </span>
                        </div>
                      </div>

                      <div>
                        <h4 className="font-bold text-slate-900 text-base">
                          {ap.patient_name}
                        </h4>
                        <div className="text-xs text-slate-600 mt-1 flex flex-col gap-0.5">
                          <span className="flex items-center gap-1">
                            <span className="text-slate-400">Doctor:</span>
                            <span className="font-medium text-slate-800">{ap.doctor}</span>
                          </span>
                          <span className="flex items-center gap-1">
                            <span className="text-slate-400">Reason:</span>
                            <span className="text-slate-700">{ap.reason || 'General'}</span>
                          </span>
                        </div>
                      </div>
                    </div>

                    {hasActions && (
                      <div className="pt-2 border-t border-slate-100 flex items-center justify-end gap-2 text-xs">
                        {showEdit && (
                          <button
                            type="button"
                            onClick={() => onEdit(ap)}
                            className="px-2.5 py-1 rounded text-blue-600 hover:bg-blue-50 font-medium transition"
                          >
                            Edit
                          </button>
                        )}
                        {showCancel && (
                          <button
                            type="button"
                            onClick={() => onCancel(ap)}
                            className="px-2.5 py-1 rounded text-amber-600 hover:bg-amber-50 font-medium transition"
                          >
                            Cancel
                          </button>
                        )}
                        {showDelete && (
                          <button
                            type="button"
                            onClick={() => onDelete(ap)}
                            className="px-2.5 py-1 rounded text-red-600 hover:bg-red-50 font-medium transition"
                          >
                            Delete
                          </button>
                        )}
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          </section>
        )
      })}
    </div>
  )
}
