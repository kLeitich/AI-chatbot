import { useMemo } from 'react'

function startOfMonth(date) {
  const d = new Date(date.getFullYear(), date.getMonth(), 1)
  return d
}
function endOfMonth(date) {
  const d = new Date(date.getFullYear(), date.getMonth() + 1, 0)
  return d
}
function startOfWeek(date) {
  const d = new Date(date)
  const day = d.getDay() // 0 Sun ... 6 Sat
  d.setDate(d.getDate() - day)
  d.setHours(0, 0, 0, 0)
  return d
}
function addDays(date, n) {
  const d = new Date(date)
  d.setDate(d.getDate() + n)
  return d
}
function addMonths(date, n) {
  const d = new Date(date)
  d.setMonth(d.getMonth() + n)
  return d
}
function ymd(d) {
  return d.toISOString().slice(0, 10)
}
function monthLabel(date) {
  return date.toLocaleString(undefined, { month: 'long', year: 'numeric' })
}

export default function Calendar({ date = new Date(), appointments = [], onAdd, onEdit, onDelete, onChangeDate }) {
  const monthStart = startOfMonth(date)
  const monthEnd = endOfMonth(date)
  const gridStart = startOfWeek(monthStart)
  const gridDays = useMemo(() => {
    const days = []
    for (let i = 0; i < 42; i++) {
      days.push(addDays(gridStart, i))
    }
    return days
  }, [gridStart])

  const apByDate = useMemo(() => {
    const map = {}
    for (const ap of appointments) {
      const key = ap.date
      if (!map[key]) map[key] = []
      map[key].push(ap)
    }
    return map
  }, [appointments])

  const isSameMonth = (d) => d.getMonth() === date.getMonth()

  return (
    <div className="border rounded bg-white shadow">
      <div className="flex items-center justify-between px-3 py-2 border-b">
        <div className="inline-flex gap-2">
          <button className="px-2 py-1 border rounded" onClick={() => onChangeDate && onChangeDate(addMonths(date, -1))}>←</button>
          <button className="px-2 py-1 border rounded" onClick={() => onChangeDate && onChangeDate(new Date())}>Today</button>
          <button className="px-2 py-1 border rounded" onClick={() => onChangeDate && onChangeDate(addMonths(date, 1))}>→</button>
        </div>
        <div className="text-sm font-semibold">{monthLabel(date)}</div>
        <div className="w-20" />
      </div>
      <div className="grid grid-cols-7 text-xs font-medium bg-gray-100 border-b">
        {['Sun','Mon','Tue','Wed','Thu','Fri','Sat'].map((d) => (
          <div key={d} className="px-2 py-2">{d}</div>
        ))}
      </div>
      <div className="grid grid-cols-7">
        {gridDays.map((d, idx) => {
          const key = ymd(d)
          const dayAps = apByDate[key] || []
          return (
            <div key={idx} className={`border p-2 h-32 overflow-y-auto ${isSameMonth(d) ? 'bg-white' : 'bg-gray-50 text-gray-400'}`}>
              <div className="flex items-center justify-between mb-1">
                <div className="text-xs font-semibold">{d.getDate()}</div>
                <button className="text-xs text-blue-600" onClick={() => onAdd && onAdd({ date: key, time: '', patient_name: '', doctor: '', reason: '', status: 'pending' })}>+ Add</button>
              </div>
              <div className="space-y-1">
                {dayAps.map((ap) => (
                  <div key={ap.id} className="text-xs bg-blue-50 border border-blue-200 rounded px-2 py-1 flex items-center justify-between">
                    <div className="truncate">
                      <span className="font-medium">{ap.time}</span> • {ap.patient_name} ({ap.doctor})
                    </div>
                    <div className="flex gap-1 ml-2 flex-shrink-0">
                      <button className="text-amber-600" title="Edit" onClick={() => onEdit && onEdit(ap)}>✏️</button>
                      <button className="text-red-600" title="Delete" onClick={() => onDelete && onDelete(ap)}>🗑️</button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
