import { useEffect, useMemo, useState, useRef } from 'react'

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
function addWeeks(date, n) {
  return addDays(date, n * 7)
}
function ymd(d) {
  return d.toISOString().slice(0, 10)
}
function monthLabel(date) {
  return date.toLocaleString(undefined, { month: 'long', year: 'numeric' })
}
function weekLabel(date) {
  const start = startOfWeek(date)
  const end = addDays(start, 6)
  const fmt = (x) => x.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  return `${fmt(start)} - ${fmt(end)}`
}

const HOURS = Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:00`)
const SLOT_PX = 48

function weekdayWithDate(d) {
  const name = d.toLocaleDateString(undefined, { weekday: 'short' })
  return `${name} ${d.getDate()}`
}

// Lightweight hash to color appointments consistently by doctor
function colorFor(ap) {
  if (!ap) return 'bg-sky-500'
  if (ap.status === 'cancelled') return 'bg-gray-500'
  if (ap.status === 'confirmed') return 'bg-emerald-500'
  const key = (ap.doctor || ap.patient_name || '').toLowerCase()
  let hash = 0
  for (let i = 0; i < key.length; i++) hash = (hash * 31 + key.charCodeAt(i)) >>> 0
  const palette = ['bg-sky-500','bg-indigo-500','bg-purple-500','bg-pink-500','bg-rose-500','bg-orange-500','bg-amber-500','bg-lime-500','bg-teal-500','bg-cyan-500']
  return palette[hash % palette.length]
}

export default function Calendar({ date = new Date(), view = 'month', appointments = [], onAdd, onEdit, onDelete, onChangeDate }) {
  const [now, setNow] = useState(new Date())
  const scrollRef = useRef(null)
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 60 * 1000)
    return () => clearInterval(id)
  }, [])

  const monthStart = startOfMonth(date)
  const gridStart = view === 'month' ? startOfWeek(monthStart) : startOfWeek(date)
  const numCells = view === 'month' ? 42 : 7

  const gridDays = useMemo(() => {
    const days = []
    for (let i = 0; i < numCells; i++) {
      days.push(addDays(gridStart, i))
    }
    return days
  }, [gridStart, numCells])

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

  const headerLabel = view === 'month' ? monthLabel(date) : weekLabel(date)
  const goPrev = () => onChangeDate && onChangeDate(view === 'month' ? addMonths(date, -1) : addWeeks(date, -1))
  const goNext = () => onChangeDate && onChangeDate(view === 'month' ? addMonths(date, 1) : addWeeks(date, 1))

  const minuteOfDay = now.getHours() * 60 + now.getMinutes()
  const nowTopPx = (minuteOfDay / (24 * 60)) * (HOURS.length * SLOT_PX)

  useEffect(() => {
    if (view !== 'week') return
    const start = new Date(gridStart)
    const end = addDays(start, 7)
    if (now >= start && now < end && scrollRef.current) {
      const target = Math.max(0, nowTopPx - 200)
      scrollRef.current.scrollTo({ top: target, behavior: 'smooth' })
    }
  }, [view, gridStart, nowTopPx, now])

  return (
    <div className="border rounded bg-white text-gray-900 shadow">
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-200">
        <div className="inline-flex gap-2">
          <button className="px-2 py-1 border rounded border-gray-200 hover:bg-gray-50" onClick={goPrev}>←</button>
          <button className="px-2 py-1 border rounded border-gray-200 hover:bg-gray-50" onClick={() => onChangeDate && onChangeDate(new Date())}>Today</button>
          <button className="px-2 py-1 border rounded border-gray-200 hover:bg-gray-50" onClick={goNext}>→</button>
        </div>
        <div className="text-sm font-semibold">{headerLabel}</div>
        <div className="w-20" />
      </div>

      {view === 'month' ? (
        <>
          <div className="grid grid-cols-7 text-xs font-medium bg-gray-100 border-b border-gray-200">
            {Array.from({ length: 7 }).map((_, i) => {
              const d = addDays(gridStart, i)
              return <div key={i} className="px-2 py-2">{weekdayWithDate(d)}</div>
            })}
          </div>
          <div className="grid grid-cols-7">
            {gridDays.map((d, idx) => {
              const key = ymd(d)
              const dayAps = apByDate[key] || []
              return (
                <div key={idx} className={`border p-2 h-32 overflow-y-auto border-gray-200 ${view === 'month' && !isSameMonth(d) ? 'bg-gray-50 text-gray-400' : 'bg-white'}`}>
                  <div className="flex items-center justify-between mb-1">
                    <div className="text-xs font-semibold text-gray-700">{d.getDate()}</div>
                  </div>
                  <div className="space-y-1">
                    {dayAps.map((ap) => (
                      <div key={ap.id ?? `${ap.date}-${ap.time}-${ap.patient_name}`} className={`text-xs ${colorFor(ap)} text-white/90 rounded px-2 py-1 flex items-center justify-between`}>
                        <div className="truncate"><span className="font-medium">{ap.time}</span> • {ap.patient_name} ({ap.doctor})</div>
                        <div className="flex gap-1 ml-2 flex-shrink-0"><button className="opacity-90 hover:opacity-100" title="Edit" onClick={() => onEdit && onEdit(ap)}>✏️</button><button className="opacity-90 hover:opacity-100" title="Delete" onClick={() => onDelete && onDelete(ap)}>🗑️</button></div>
                      </div>
                    ))}
                  </div>
                </div>
              )
            })}
          </div>
        </>
      ) : (
        <div className="">
          <div className="grid" style={{ gridTemplateColumns: '100px repeat(7, 1fr)' }}>
            <div className="p-2 text-xs font-medium bg-gray-100 border-b border-gray-200">Time</div>
            {Array.from({ length: 7 }).map((_, i) => {
              const d = addDays(gridStart, i)
              return <div key={i} className="p-2 text-xs font-medium bg-gray-100 border-b border-gray-200">{weekdayWithDate(d)}</div>
            })}
          </div>
          <div className="relative max-h-[70vh] overflow-y-auto" ref={scrollRef}>
            {ymd(now) >= ymd(gridStart) && ymd(now) <= ymd(addDays(gridStart, 6)) && (
              <div className="absolute left-[100px] right-0" style={{ top: nowTopPx }}><div className="h-px bg-red-500" /></div>
            )}
            <div className="grid" style={{ gridTemplateColumns: '100px repeat(7, 1fr)' }}>
              {HOURS.map((h) => (
                <>
                  <div key={`h-${h}`} className="border-r p-2 text-xs h-12 leading-[48px] text-gray-500 border-gray-200">{h}</div>
                  {Array.from({ length: 7 }).map((_, dayIdx) => {
                    const day = addDays(gridStart, dayIdx)
                    const key = ymd(day)
                    const items = (apByDate[key] || []).filter((ap) => ap.time?.startsWith(h))
                    const onCellClick = (e) => {
                      if (!onAdd) return
                      const rect = e.currentTarget.getBoundingClientRect()
                      const y = e.clientY - rect.top
                      const minute = y < rect.height / 2 ? '00' : '30'
                      const hh = h.slice(0, 2)
                      onAdd({ date: key, time: `${hh}:${minute}`, patient_name: '', doctor: '', reason: '', status: 'pending' })
                    }
                    return (
                      <div key={`${key}-${h}`} className="border h-12 p-1 cursor-crosshair hover:bg-gray-50/70 border-gray-200" onClick={onCellClick}>
                        <div className="flex flex-col gap-1 pointer-events-none">
                          {items.map((ap) => (
                            <div key={ap.id ?? `${ap.date}-${ap.time}-${ap.patient_name}`} className={`text-xs ${colorFor(ap)} text-white/90 rounded px-2 py-1 flex items-center justify-between`}>
                              <div className="truncate"><span className="font-medium">{ap.time}</span> • {ap.patient_name}</div>
                              <div className="flex gap-1 ml-2 flex-shrink-0"><button className="pointer-events-auto opacity-90 hover:opacity-100" title="Edit" onClick={(ev) => { ev.stopPropagation(); onEdit && onEdit(ap) }}>✏️</button><button className="pointer-events-auto opacity-90 hover:opacity-100" title="Delete" onClick={(ev) => { ev.stopPropagation(); onDelete && onDelete(ap) }}>🗑️</button></div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )
                  })}
                </>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
