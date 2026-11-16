"use client"
import React, { Suspense, useEffect, useState } from 'react'
import { useParams, useSearchParams } from 'next/navigation'
import api from '../../../lib/api'
import Chatbot from '../../../components/Chatbot'

function BookPageContent() {
  const params = useParams()
  const searchParams = useSearchParams()
  const tenant = params?.tenant

  const [config, setConfig] = useState(null)
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)

  const doctor = searchParams?.get('doctor')
  const reason = searchParams?.get('reason')
  const source = searchParams?.get('source')

  useEffect(() => {
    if (!tenant) return
    let active = true
    setLoading(true)
    api.get(`/t/${tenant}/public-config`)
      .then((res) => {
        if (!active) return
        setConfig(res.data)
        setLoading(false)
      })
      .catch((err) => {
        if (!active) return
        if (err.response?.status === 404 || err.response?.data?.code === 'NOT_FOUND') {
          setNotFound(true)
        } else {
          setNotFound(true)
        }
        setLoading(false)
      })
    return () => { active = false }
  }, [tenant])

  if (loading) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <div className="flex items-center gap-2 text-slate-500 font-medium text-sm">
          <span className="w-2.5 h-2.5 bg-blue-600 rounded-full animate-bounce" />
          Loading clinic booking...
        </div>
      </div>
    )
  }

  if (notFound) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <div className="max-w-md w-full bg-white p-8 rounded-2xl shadow-sm border text-center space-y-3">
          <div className="text-4xl" aria-hidden="true">🏥</div>
          <h1 className="text-2xl font-bold text-slate-800">Clinic not found</h1>
          <p className="text-sm text-slate-600">The clinic you are looking for does not exist or may have been moved.</p>
        </div>
      </div>
    )
  }

  if (!config?.booking_enabled) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <div className="max-w-md w-full bg-white p-8 rounded-2xl shadow-sm border text-center space-y-4">
          <div className="text-4xl" aria-hidden="true">📅</div>
          <h1 className="text-2xl font-bold text-slate-800">Online booking is unavailable</h1>
          <p className="text-sm text-slate-600">
            Online booking is currently disabled for {config?.name || 'this clinic'}.
          </p>
          {config?.emergency_number && (
            <div className="pt-2 border-t border-slate-100">
              <p className="text-xs text-slate-500 mb-1">For urgent inquiries or emergency support:</p>
              <a
                href={`tel:${config.emergency_number}`}
                className="inline-flex items-center gap-1.5 px-4 py-2 bg-red-600 hover:bg-red-700 text-white font-bold rounded-lg text-sm transition shadow-sm"
              >
                📞 Call {config.emergency_number}
              </a>
            </div>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-slate-50 py-8 px-4">
      <div className="max-w-3xl mx-auto mb-4 text-center">
        <h1 className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight">
          {config.name}
        </h1>
        <p className="text-sm text-slate-500 mt-1">Book your appointment with our medical team</p>
      </div>
      <Chatbot
        tenant={tenant}
        publicConfig={config}
        initialContext={{ doctor, reason, source }}
      />
    </div>
  )
}

export default function BookTenantPage() {
  return (
    <Suspense fallback={<div className="min-h-screen bg-slate-50" />}>
      <BookPageContent />
    </Suspense>
  )
}
