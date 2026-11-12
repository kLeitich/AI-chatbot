"use client"
import { useEffect, useRef, useState } from 'react'
import api from '../lib/api'

function Toast({ text, onClose }) {
  useEffect(() => {
    const id = setTimeout(onClose, 2500)
    return () => clearTimeout(id)
  }, [onClose])
  return (
    <div className="fixed top-4 right-4 bg-gray-900 text-white px-4 py-2 rounded shadow z-50">
      {text}
    </div>
  )
}

function EmergencyCard({ text, emergencyNumber }) {
  const primaryNumber = emergencyNumber && emergencyNumber.trim() ? emergencyNumber.trim() : '999'
  return (
    <div className="w-full bg-red-700 text-white p-4 rounded-xl border-2 border-red-900 shadow-lg space-y-3" role="alert">
      <div className="flex items-center gap-2">
        <span className="text-2xl" aria-hidden="true">🚨</span>
        <h3 className="text-lg font-bold tracking-wide uppercase">Medical Emergency Detected</h3>
      </div>
      <p className="text-sm text-red-50 font-medium whitespace-pre-line leading-relaxed">{text}</p>
      <div className="pt-2 border-t border-red-600/60 flex flex-wrap gap-2">
        {primaryNumber !== '999' && primaryNumber !== '112' && (
          <a
            href={`tel:${primaryNumber}`}
            className="inline-flex items-center gap-2 px-4 py-2 bg-white text-red-800 font-bold rounded-lg shadow hover:bg-red-50 text-sm focus:outline-none focus:ring-2 focus:ring-white transition"
          >
            📞 Call Clinic ({primaryNumber})
          </a>
        )}
        <a
          href="tel:999"
          className="inline-flex items-center gap-2 px-4 py-2 bg-white text-red-800 font-bold rounded-lg shadow hover:bg-red-50 text-sm focus:outline-none focus:ring-2 focus:ring-white transition"
        >
          📞 Call 999
        </a>
        <a
          href="tel:112"
          className="inline-flex items-center gap-2 px-4 py-2 bg-red-900 text-white font-bold rounded-lg shadow hover:bg-red-950 text-sm border border-red-600 focus:outline-none focus:ring-2 focus:ring-white transition"
        >
          📞 Call 112
        </a>
      </div>
    </div>
  )
}

function MessageBubble({ role, text, emergency, emergencyNumber }) {
  const isUser = role === 'user'

  if (emergency) {
    return (
      <div className="flex justify-start w-full my-2">
        <EmergencyCard text={text} emergencyNumber={emergencyNumber} />
      </div>
    )
  }

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
      <div className={`max-w-[80%] px-4 py-2 rounded-lg shadow text-sm ${isUser ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-900'}`}>
        {text}
      </div>
    </div>
  )
}

export default function Chatbot({ tenant = 'default' }) {
  const initialGreeting = { role: 'ai', text: 'Hi! I can book a doctor appointment. Tell me details.' }
  const [messages, setMessages] = useState([initialGreeting])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [toast, setToast] = useState(null)
  const [sessionId, setSessionId] = useState('')
  const bottomRef = useRef(null)

  // Initialize sessionId from localStorage if previously established
  useEffect(() => {
    if (typeof window === 'undefined') return
    const k = 'chat_session_id_' + tenant
    const v = localStorage.getItem(k)
    if (v) setSessionId(v)
  }, [tenant])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, loading])

  const startNewConversation = () => {
    setMessages([initialGreeting])
    setInput('')
    setSessionId('')
    if (typeof window !== 'undefined') {
      localStorage.removeItem('chat_session_id_' + tenant)
    }
  }

  const isEmergencyReply = (txt) => {
    if (!txt) return false
    return /\b(999|112|emergency|immediately|urgent medical)\b/i.test(txt)
  }

  const send = async () => {
    if (!input.trim()) return
    const userText = input.trim()
    setMessages((prev) => [...prev, { role: 'user', text: userText }])
    setInput('')
    setLoading(true)
    try {
      const payload = { message: userText }
      if (sessionId) payload.session_id = sessionId
      const res = await api.post(`/t/${tenant}/chat`, payload)
      const { message, reply, appointment, session_id, emergency, emergency_category, emergency_number } = res.data || {}
      if (session_id) {
        setSessionId(session_id)
        localStorage.setItem('chat_session_id_' + tenant, session_id)
      }
      const aiText = reply || message || "Hmm, I didn’t catch that."
      const isEmergency = Boolean(emergency) || isEmergencyReply(aiText)

      setMessages((prev) => [
        ...prev,
        {
          role: 'ai',
          text: aiText,
          emergency: isEmergency,
          emergencyNumber: emergency_number || '999',
          emergencyCategory: emergency_category,
        },
      ])
      if (appointment) setToast('Appointment created!')
    } catch (e) {
      setMessages((prev) => [
        ...prev,
        {
          role: 'ai',
          text: "I'm having trouble connecting right now. Please check your connection and try sending your message again.",
        },
      ])
    } finally {
      setLoading(false)
    }
  }

  const onKey = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      send()
    }
  }

  return (
    <div className="max-w-3xl mx-auto p-4">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-semibold text-gray-900">AI Doctor Appointment Chatbot</h1>
        <button
          onClick={startNewConversation}
          className="text-xs sm:text-sm px-3 py-1.5 rounded-lg border border-gray-300 text-gray-700 hover:bg-gray-50 transition"
        >
          Start new conversation
        </button>
      </div>

      <div className="border rounded bg-white h-[60vh] p-4 overflow-y-auto space-y-3 shadow">
        {messages.map((m, i) => (
          <MessageBubble
            key={i}
            role={m.role}
            text={m.text}
            emergency={m.emergency}
            emergencyNumber={m.emergencyNumber}
          />
        ))}
        {loading && (
          <div className="flex items-center gap-2 text-gray-500">
            <span className="animate-pulse w-2 h-2 bg-gray-400 rounded-full" />
            <span className="animate-pulse w-2 h-2 bg-gray-400 rounded-full" />
            <span className="animate-pulse w-2 h-2 bg-gray-400 rounded-full" />
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      <div className="mt-3 flex gap-2">
        <textarea
          value={input}
          maxLength={1000}
          onKeyDown={onKey}
          onChange={(e) => setInput(e.target.value)}
          placeholder="e.g., Book me with Dr. Kim tomorrow at 10am for checkup"
          className="flex-1 border rounded p-3 focus:outline-none focus:ring"
          rows={2}
        />
        <button
          onClick={send}
          className="px-4 py-2 rounded bg-blue-600 text-white disabled:opacity-60"
          disabled={loading}
        >
          Send
        </button>
      </div>
      {toast && <Toast text={toast} onClose={() => setToast(null)} />}
    </div>
  )
}
