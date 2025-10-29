"use client"
import { useEffect, useMemo, useRef, useState } from 'react'
import api from '../lib/api'

function uuid() {
  return crypto.randomUUID?.() || Math.random().toString(36).slice(2)
}

function Toast({ text, onClose }) {
  useEffect(() => {
    const id = setTimeout(onClose, 2500)
    return () => clearTimeout(id)
  }, [onClose])
  return (
    <div className="fixed top-4 right-4 bg-gray-900 text-white px-4 py-2 rounded shadow">
      {text}
    </div>
  )
}

export default function Chatbot() {
  const [messages, setMessages] = useState([{ role: 'ai', text: 'Hi! I can book a doctor appointment. Tell me details.' }])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [toast, setToast] = useState(null)
  const [sessionId, setSessionId] = useState('')
  const bottomRef = useRef(null)

  // Initialize sessionId on client only
  useEffect(() => {
    if (typeof window === 'undefined') return
    const k = 'chat_session_id'
    let v = localStorage.getItem(k)
    if (!v) { v = uuid(); localStorage.setItem(k, v) }
    setSessionId(v)
  }, [])

  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages, loading])

  const send = async () => {
    if (!input.trim()) return
    const userText = input.trim()
    setMessages((prev) => [...prev, { role: 'user', text: userText }])
    setInput('')
    setLoading(true)
    try {
      const payload = { message: userText }
      if (sessionId) payload.session_id = sessionId
      const res = await api.post('/chat', payload)
      const { message, reply, appointment } = res.data || {}
      const aiText = reply || message || "Hmm, I didn’t catch that."
      setMessages((prev) => [...prev, { role: 'ai', text: aiText }])
      if (appointment) setToast('Appointment created!')
    } catch (e) {
      setMessages((prev) => [...prev, { role: 'ai', text: "Sorry, I couldn't process that." }])
    } finally {
      setLoading(false)
    }
  }

  const onKey = (e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() } }

  return (
    <div className="max-w-3xl mx-auto p-4">
      <h1 className="text-2xl font-semibold mb-4">AI Doctor Appointment Chatbot</h1>
      <div className="border rounded bg-white h-[60vh] p-4 overflow-y-auto space-y-3 shadow">
        {messages.map((m, i) => (<MessageBubble key={i} role={m.role} text={m.text} />))}
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
        <textarea value={input} onKeyDown={onKey} onChange={(e) => setInput(e.target.value)} placeholder="e.g., Book me with Dr. Kim tomorrow at 10am for checkup" className="flex-1 border rounded p-3 focus:outline-none focus:ring" rows={2} />
        <button onClick={send} className="px-4 py-2 rounded bg-blue-600 text-white disabled:opacity-60" disabled={loading}>Send</button>
      </div>
      {toast && <Toast text={toast} onClose={() => setToast(null)} />}
    </div>
  )
}

function MessageBubble({ role, text }) {
  const isUser = role === 'user'
  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
      <div className={`max-w-[80%] px-4 py-2 rounded-lg shadow text-sm ${isUser ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-900'}`}>
        {text}
      </div>
    </div>
  )
}
