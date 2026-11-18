"use client"
import React, { Suspense, useEffect, useState } from 'react'
import { useParams, useSearchParams } from 'next/navigation'
import api from '../../../lib/api'
import Chatbot from '../../../components/Chatbot'

function EmbedPageContent() {
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
      <div className="h-screen w-full flex items-center justify-center p-4 bg-white">
        <div className="flex items-center gap-2 text-slate-500 font-medium text-xs">
          <span className="w-2 h-2 bg-blue-600 rounded-full animate-bounce" />
          Loading...
        </div>
      </div>
    )
  }

  if (notFound) {
    return (
      <div className="h-screen w-full flex flex-col items-center justify-center p-6 text-center bg-white space-y-2">
        <div className="text-3xl" aria-hidden="true">🏥</div>
        <h1 className="text-lg font-bold text-slate-800">Clinic not found</h1>
        <p className="text-xs text-slate-500">The requested clinic link is unavailable.</p>
      </div>
    )
  }

  if (!config?.booking_enabled) {
    return (
      <div className="h-screen w-full flex flex-col items-center justify-center p-6 text-center bg-white space-y-3">
        <div className="text-3xl" aria-hidden="true">📅</div>
        <h1 className="text-lg font-bold text-slate-800">Online booking is unavailable</h1>
        <p className="text-xs text-slate-600 max-w-xs">
          Online booking is currently disabled for {config?.name || 'this clinic'}.
        </p>
        {config?.emergency_number && (
          <a
            href={`tel:${config.emergency_number}`}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-red-600 hover:bg-red-700 text-white font-bold rounded-lg text-xs transition shadow-sm"
          >
            📞 Call {config.emergency_number}
          </a>
        )}
      </div>
    )
  }

  return (
    <div className="h-screen w-full flex flex-col bg-white overflow-hidden">
      <Chatbot
        tenant={tenant}
        publicConfig={config}
        initialContext={{ doctor, reason, source }}
        isEmbed={true}
      />
    </div>
  )
}

export default function EmbedTenantPage() {
  return (
    <Suspense fallback={<div className="h-screen w-full bg-white" />}>
      <EmbedPageContent />
    </Suspense>
  )
}
