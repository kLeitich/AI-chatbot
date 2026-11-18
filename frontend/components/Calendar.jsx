"use client"
import { useEffect, useMemo, useState, useRef } from 'react'

function startOfMonth(date) { const d = new Date(date.getFullYear(), date.getMonth(), 1); return d }
function startOfWeek(date) { const d = new Date(date); const day = d.getDay(); d.setDate(d.getDate() - day); d.setHours(0,0,0,0); return d }
function addDays(date, n) { const d = new Date(date); d.setDate(d.getDate() + n); return d }
function addMonths(date, n) { const d = new Date(date); d.setMonth(d.getMonth() + n); return d }
function addWeeks(date, n) { return addDays(date, n * 7) }
function ymd(d) { const year=d.getFullYear(); const month=String(d.getMonth()+1).padStart(2,'0'); const day=String(d.getDate()).padStart(2,'0'); return `${year}-${month}-${day}` }
function monthLabel(date) { return date.toLocaleString(undefined, { month: 'long', year: 'numeric' }) }
function weekLabel(date) { const s=startOfWeek(date); const e=addDays(s,6); const fmt=(x)=>x.toLocaleDateString(undefined,{month:'short',day:'numeric'}); return `${fmt(s)} - ${fmt(e)}` }
function dayLabel(date) { return date.toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' }) }

const HOURS = Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:00`)
const SLOT_PX = 48
function weekdayWithDate(d){ const name=d.toLocaleDateString(undefined,{weekday:'short'}); return `${name} ${d.getDate()}` }
function colorFor(ap){
  if(!ap) return 'bg-sky-500'
  if(ap.status==='cancelled') return 'bg-gray-400 line-through'
  if(ap.status==='confirmed') return 'bg-emerald-600'
  if(ap.status==='pending') return 'bg-amber-500'
  if(ap.status==='completed') return 'bg-blue-600'
  if(ap.status==='no_show') return 'bg-purple-600'
  return 'bg-sky-500'
}

export default function Calendar({
  date = new Date(),
  view = 'month',
  appointments = [],
  onAdd,
  onEdit,
  onCancel,
  onDelete,
  onChangeDate,
  canCreate = true,
  canEdit = true,
  canCancel = false,
  canDelete = true,
}) {
  const [now, setNow] = useState(new Date())
  const scrollRef = useRef(null)
  useEffect(()=>{ const id=setInterval(()=>setNow(new Date()),60*1000); return ()=>clearInterval(id) },[])

  const monthStart=startOfMonth(date); const gridStart=view==='month'?startOfWeek(monthStart):startOfWeek(date); const numCells=view==='month'?42:7
  const gridDays=useMemo(()=>{ const days=[]; for(let i=0;i<numCells;i++) days.push(addDays(gridStart,i)); return days },[gridStart,numCells])
  const apByDate=useMemo(()=>{ const map={}; for(const ap of appointments){ const key=ap.date; if(!map[key]) map[key]=[]; map[key].push(ap)} return map },[appointments])

  const distinctDoctors = useMemo(() => {
    const set = new Set()
    for (const ap of appointments) {
      if (ap.doctor && ap.doctor.trim()) {
        set.add(ap.doctor.trim())
      }
    }
    const arr = Array.from(set).sort()
    return arr.length > 0 ? arr : ['All Doctors']
  }, [appointments])

  const isSameMonth=(d)=>d.getMonth()===date.getMonth()
  const headerLabel=view==='month'?monthLabel(date):view==='day'?dayLabel(date):weekLabel(date)
  const goPrev=()=>onChangeDate&&onChangeDate(view==='month'?addMonths(date,-1):view==='day'?addDays(date,-1):addWeeks(date,-1))
  const goNext=()=>onChangeDate&&onChangeDate(view==='month'?addMonths(date,1):view==='day'?addDays(date,1):addWeeks(date,1))

  const minuteOfDay=now.getHours()*60+now.getMinutes(); const nowTopPx=(minuteOfDay/(24*60))*(HOURS.length*SLOT_PX)
  useEffect(()=>{
    if(view!=='week' && view!=='day') return;
    if(scrollRef.current && typeof scrollRef.current.scrollTo === 'function'){
      const target=Math.max(0,nowTopPx-200);
      scrollRef.current.scrollTo({top:target,behavior:'smooth'})
    }
  },[view,nowTopPx])

  const canClickCell = Boolean(onAdd && canCreate)
  const currentDayKey = ymd(date)

  return (
    <div className="border rounded bg-white text-gray-900 shadow" data-testid="calendar-view">
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-200">
        <div className="inline-flex gap-2">
          <button className="px-2 py-1 border rounded border-gray-200 hover:bg-gray-50" onClick={goPrev}>←</button>
          <button className="px-2 py-1 border rounded border-gray-200 hover:bg-gray-50" onClick={()=>onChangeDate&&onChangeDate(new Date())}>Today</button>
          <button className="px-2 py-1 border rounded border-gray-200 hover:bg-gray-50" onClick={goNext}>→</button>
        </div>
        <div className="text-sm font-semibold">{headerLabel}</div>
        <div className="w-20" />
      </div>

      {view==='month'? (
        <>
          <div className="grid grid-cols-7 text-xs font-medium bg-gray-100 border-b border-gray-200">
            {Array.from({length:7}).map((_,i)=>{ const d=addDays(gridStart,i); return <div key={i} className="px-2 py-2">{weekdayWithDate(d)}</div> })}
          </div>
          <div className="grid grid-cols-7">
            {gridDays.map((d,idx)=>{
              const key=ymd(d); const dayAps=apByDate[key]||[]
              return (
                <div key={idx} className={`border p-2 h-32 overflow-y-auto border-gray-200 ${view==='month'&&!isSameMonth(d)?'bg-gray-50 text-gray-400':'bg-white'}`}>
                  <div className="flex items-center justify-between mb-1">
                    <div className="text-xs font-semibold text-gray-700">{d.getDate()}</div>
                  </div>
                  <div className="space-y-1">
                    {dayAps.map((ap)=>{
                      const showEdit = Boolean(onEdit && canEdit)
                      const showCancel = Boolean(onCancel && canCancel && ap.status !== 'cancelled')
                      const showDelete = Boolean(onDelete && canDelete)
                      const hasItemActions = showEdit || showCancel || showDelete

                      return (
                        <div key={ap.id ?? `${ap.date}-${ap.time}-${ap.patient_name}`} className={`text-xs ${colorFor(ap)} text-white/90 rounded px-2 py-1 flex items-center justify-between`}>
                          <div className="truncate"><span className="font-medium">{ap.time}</span> • {ap.patient_name} ({ap.doctor})</div>
                          {hasItemActions && (
                            <div className="flex gap-1 ml-2 flex-shrink-0">
                              {showEdit && (
                                <button className="opacity-90 hover:opacity-100" title="Edit" onClick={()=>onEdit(ap)}>✏️</button>
                              )}
                              {showCancel && (
                                <button className="opacity-90 hover:opacity-100 text-xs" title="Cancel" onClick={()=>onCancel(ap)}>❌</button>
                              )}
                              {showDelete && (
                                <button className="opacity-90 hover:opacity-100" title="Delete" onClick={()=>onDelete(ap)}>🗑️</button>
                              )}
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </div>
                </div>
              )
            })}
          </div>
        </>
      ) : view==='day' ? (
        /* DAY MODE: Hourly slots per doctor */
        <div data-testid="calendar-day-mode">
          <div
            className="grid border-b border-gray-200 bg-gray-100"
            style={{ gridTemplateColumns: `100px repeat(${distinctDoctors.length}, minmax(180px, 1fr))` }}
          >
            <div className="p-2.5 text-xs font-semibold text-gray-700 border-r border-gray-200">Time</div>
            {distinctDoctors.map((doc) => (
              <div key={doc} className="p-2.5 text-xs font-bold text-gray-900 border-r border-gray-200 truncate">
                👨‍⚕️ {doc}
              </div>
            ))}
          </div>

          <div className="relative max-h-[70vh] overflow-y-auto" ref={scrollRef}>
            {ymd(now) === currentDayKey && (
              <div className="absolute left-[100px] right-0 z-10" style={{ top: nowTopPx }}>
                <div className="h-px bg-red-500 shadow-sm" />
              </div>
            )}
            <div
              className="grid"
              style={{ gridTemplateColumns: `100px repeat(${distinctDoctors.length}, minmax(180px, 1fr))` }}
            >
              {HOURS.map((h) => (
                <div key={`day-row-${h}`} className="contents">
                  <div className="border-r border-b p-2 text-xs h-14 leading-[44px] text-gray-500 border-gray-200 bg-gray-50/50">
                    {h}
                  </div>
                  {distinctDoctors.map((doc) => {
                    const items = (apByDate[currentDayKey] || []).filter((ap) => {
                      const matchDoctor = distinctDoctors.length === 1 && distinctDoctors[0] === 'All Doctors'
                        ? true
                        : ap.doctor === doc
                      return matchDoctor && ap.time?.startsWith(h.slice(0, 2))
                    })

                    const onCellClick = (e) => {
                      if (!canClickCell) return
                      const rect = e.currentTarget.getBoundingClientRect()
                      const y = e.clientY - rect.top
                      const minute = y < rect.height / 2 ? '00' : '30'
                      const hh = h.slice(0, 2)
                      onAdd({
                        date: currentDayKey,
                        time: `${hh}:${minute}`,
                        patient_name: '',
                        doctor: doc === 'All Doctors' ? '' : doc,
                        reason: '',
                        status: 'pending',
                      })
                    }

                    return (
                      <div
                        key={`${currentDayKey}-${h}-${doc}`}
                        className={`border-r border-b min-h-14 p-1.5 border-gray-200 ${canClickCell ? 'cursor-crosshair hover:bg-gray-50/80' : 'cursor-default'}`}
                        onClick={canClickCell ? onCellClick : undefined}
                      >
                        <div className="flex flex-col gap-1 pointer-events-none">
                          {items.map((ap) => {
                            const showEdit = Boolean(onEdit && canEdit)
                            const showCancel = Boolean(onCancel && canCancel && ap.status !== 'cancelled')
                            const showDelete = Boolean(onDelete && canDelete)
                            const hasItemActions = showEdit || showCancel || showDelete

                            return (
                              <div
                                key={ap.id ?? `${ap.date}-${ap.time}-${ap.patient_name}`}
                                className={`text-xs ${colorFor(ap)} text-white/95 rounded-lg px-2.5 py-1.5 shadow-sm flex items-center justify-between`}
                              >
                                <div className="truncate">
                                  <span className="font-bold">{ap.time}</span> • {ap.patient_name}
                                  {ap.reason && <span className="text-[11px] opacity-90 block">{ap.reason}</span>}
                                </div>
                                {hasItemActions && (
                                  <div className="flex gap-1 ml-2 flex-shrink-0">
                                    {showEdit && (
                                      <button className="pointer-events-auto opacity-90 hover:opacity-100" title="Edit" onClick={(ev)=>{ev.stopPropagation(); onEdit(ap)}}>✏️</button>
                                    )}
                                    {showCancel && (
                                      <button className="pointer-events-auto opacity-90 hover:opacity-100 text-xs" title="Cancel" onClick={(ev)=>{ev.stopPropagation(); onCancel(ap)}}>❌</button>
                                    )}
                                    {showDelete && (
                                      <button className="pointer-events-auto opacity-90 hover:opacity-100" title="Delete" onClick={(ev)=>{ev.stopPropagation(); onDelete(ap)}}>🗑️</button>
                                    )}
                                  </div>
                                )}
                              </div>
                            )
                          })}
                        </div>
                      </div>
                    )
                  })}
                </div>
              ))}
            </div>
          </div>
        </div>
      ) : (
        /* WEEK MODE */
        <div className="">
          <div className="grid" style={{gridTemplateColumns:'100px repeat(7, 1fr)'}}>
            <div className="p-2 text-xs font-medium bg-gray-100 border-b border-gray-200">Time</div>
            {Array.from({length:7}).map((_,i)=>{ const d=addDays(gridStart,i); return <div key={i} className="p-2 text-xs font-medium bg-gray-100 border-b border-gray-200">{weekdayWithDate(d)}</div> })}
          </div>
          <div className="relative max-h-[70vh] overflow-y-auto" ref={scrollRef}>
            {ymd(now)>=ymd(gridStart)&&ymd(now)<=ymd(addDays(gridStart,6))&&(
              <div className="absolute left-[100px] right-0" style={{top:nowTopPx}}><div className="h-px bg-red-500" /></div>
            )}
            <div className="grid" style={{gridTemplateColumns:'100px repeat(7, 1fr)'}}>
              {HOURS.map((h)=>(
                <div key={`row-${h}`} className="contents">
                  <div key={`h-${h}`} className="border-r p-2 text-xs h-12 leading-[48px] text-gray-500 border-gray-200">{h}</div>
                  {Array.from({length:7}).map((_,dayIdx)=>{
                    const day=addDays(gridStart,dayIdx)
                    const key=ymd(day)
                    const items=(apByDate[key]||[]).filter((ap)=>ap.time?.startsWith(h.slice(0,2)))
                    const onCellClick=(e)=>{
                      if(!canClickCell) return
                      const rect=e.currentTarget.getBoundingClientRect()
                      const y=e.clientY-rect.top
                      const minute=y<rect.height/2?'00':'30'
                      const hh=h.slice(0,2)
                      onAdd({date:key,time:`${hh}:${minute}`,patient_name:'',doctor:'',reason:'',status:'pending'})
                    }
                    return (
                      <div
                        key={`${key}-${h}`}
                        className={`border h-12 p-1 border-gray-200 ${canClickCell ? 'cursor-crosshair hover:bg-gray-50/70' : 'cursor-default'}`}
                        onClick={canClickCell ? onCellClick : undefined}
                      >
                        <div className="flex flex-col gap-1 pointer-events-none">
                          {items.map((ap)=>{
                            const showEdit = Boolean(onEdit && canEdit)
                            const showCancel = Boolean(onCancel && canCancel && ap.status !== 'cancelled')
                            const showDelete = Boolean(onDelete && canDelete)
                            const hasItemActions = showEdit || showCancel || showDelete

                            return (
                              <div key={ap.id ?? `${ap.date}-${ap.time}-${ap.patient_name}`} className={`text-xs ${colorFor(ap)} text-white/90 rounded px-2 py-1 flex items-center justify-between`}>
                                <div className="truncate"><span className="font-medium">{ap.time}</span> • {ap.patient_name}</div>
                                {hasItemActions && (
                                  <div className="flex gap-1 ml-2 flex-shrink-0">
                                    {showEdit && (
                                      <button className="pointer-events-auto opacity-90 hover:opacity-100" title="Edit" onClick={(ev)=>{ev.stopPropagation(); onEdit(ap)}}>✏️</button>
                                    )}
                                    {showCancel && (
                                      <button className="pointer-events-auto opacity-90 hover:opacity-100 text-xs" title="Cancel" onClick={(ev)=>{ev.stopPropagation(); onCancel(ap)}}>❌</button>
                                    )}
                                    {showDelete && (
                                      <button className="pointer-events-auto opacity-90 hover:opacity-100" title="Delete" onClick={(ev)=>{ev.stopPropagation(); onDelete(ap)}}>🗑️</button>
                                    )}
                                  </div>
                                )}
                              </div>
                            )
                          })}
                        </div>
                      </div>
                    )
                  })}
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
