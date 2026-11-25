"use client"
import { useEffect, useState, useCallback } from 'react'
import api from '../lib/api'

export default function IntegrationsSettings({ tenant = 'default' }) {
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testingChatwoot, setTestingChatwoot] = useState(false)
  const [copied, setCopied] = useState(false)
  const [copiedGlobal, setCopiedGlobal] = useState(false)
  const [msg, setMsg] = useState({ error: '', success: '' })
  const [testResult, setTestResult] = useState(null)

  const [sessions, setSessions] = useState([])
  const [branches, setBranches] = useState([])
  const [newBranch, setNewBranch] = useState({ name: '', address: '', phone: '' })

  const [form, setForm] = useState({
    // Provider Profile & Omnichannel AI Preferences
    provider_type: 'clinic',
    whatsapp_number: '',
    address: '',
    phone: '',
    business_hours: 'Mon-Fri 08:00-17:00, Sat 09:00-13:00',
    cancellation_policy: '',
    rescheduling_policy: '',
    ai_receptionist_enabled: true,
    marketplace_enabled: false,
    handoff_keywords: 'human,agent,receptionist,speak to someone',

    // Chatwoot & WhatsApp
    chatwoot_enabled: false,
    chatwoot_base_url: 'https://app.chatwoot.com',
    chatwoot_api_token: '',
    chatwoot_account_id: '',
    chatwoot_inbox_id: '',
    chatwoot_webhook_secret: '',

    // Google Calendar
    google_calendar_enabled: false,
    google_calendar_id: 'primary',
    google_calendar_token: '',
    google_client_id: '',
    google_client_secret: '',

    // Clerk SSO
    clerk_enabled: false,
    clerk_publishable_key: '',
    clerk_secret_key: '',
  })

  const [showTokens, setShowTokens] = useState({
    chatwoot: false,
    chatwootSecret: false,
    gcalendar: false,
    clerk: false,
  })

  const fetchSettingsAndMonitorData = useCallback(async () => {
    setLoading(true)
    setMsg({ error: '', success: '' })
    try {
      const [settingsRes, sessRes, branchRes] = await Promise.all([
        api.get(`/t/${tenant}/admin/settings`),
        api.get(`/t/${tenant}/admin/integrations/chatwoot/sessions`).catch(() => ({ data: [] })),
        api.get(`/t/${tenant}/admin/branches`).catch(() => ({ data: [] })),
      ])

      const d = settingsRes.data || {}
      setForm({
        provider_type: d.provider_type || 'clinic',
        whatsapp_number: d.whatsapp_number || '',
        address: d.address || '',
        phone: d.phone || '',
        business_hours: d.business_hours || 'Mon-Fri 08:00-17:00, Sat 09:00-13:00',
        cancellation_policy: d.cancellation_policy || '',
        rescheduling_policy: d.rescheduling_policy || '',
        ai_receptionist_enabled: d.ai_receptionist_enabled !== undefined ? Boolean(d.ai_receptionist_enabled) : true,
        marketplace_enabled: Boolean(d.marketplace_enabled),
        handoff_keywords: d.handoff_keywords || 'human,agent,receptionist,speak to someone',

        chatwoot_enabled: Boolean(d.chatwoot_enabled),
        chatwoot_base_url: d.chatwoot_base_url || 'https://app.chatwoot.com',
        chatwoot_api_token: d.chatwoot_api_token || '',
        chatwoot_account_id: d.chatwoot_account_id ? String(d.chatwoot_account_id) : '',
        chatwoot_inbox_id: d.chatwoot_inbox_id ? String(d.chatwoot_inbox_id) : '',
        chatwoot_webhook_secret: d.chatwoot_webhook_secret || '',

        google_calendar_enabled: Boolean(d.google_calendar_enabled),
        google_calendar_id: d.google_calendar_id || 'primary',
        google_calendar_token: d.google_calendar_token || '',
        google_client_id: d.google_client_id || '',
        google_client_secret: d.google_client_secret || '',

        clerk_enabled: Boolean(d.clerk_enabled),
        clerk_publishable_key: d.clerk_publishable_key || '',
        clerk_secret_key: d.clerk_secret_key || '',
      })
      setSessions(Array.isArray(sessRes.data) ? sessRes.data : [])
      setBranches(Array.isArray(branchRes.data) ? branchRes.data : [])
    } catch (err) {
      setMsg({
        error: err?.response?.data?.message || 'Failed to load integration settings',
        success: '',
      })
    } finally {
      setLoading(false)
    }
  }, [tenant])

  useEffect(() => {
    fetchSettingsAndMonitorData()
  }, [fetchSettingsAndMonitorData])

  const handleSave = async (e) => {
    e.preventDefault()
    setSaving(true)
    setMsg({ error: '', success: '' })

    const payload = {
      provider_type: form.provider_type,
      whatsapp_number: form.whatsapp_number.trim(),
      address: form.address.trim(),
      phone: form.phone.trim(),
      business_hours: form.business_hours.trim(),
      cancellation_policy: form.cancellation_policy.trim(),
      rescheduling_policy: form.rescheduling_policy.trim(),
      ai_receptionist_enabled: form.ai_receptionist_enabled,
      marketplace_enabled: form.marketplace_enabled,
      handoff_keywords: form.handoff_keywords.trim(),

      chatwoot_enabled: form.chatwoot_enabled,
      chatwoot_base_url: form.chatwoot_base_url.trim(),
      chatwoot_api_token: form.chatwoot_api_token.trim(),
      chatwoot_account_id: form.chatwoot_account_id ? parseInt(form.chatwoot_account_id, 10) : 0,
      chatwoot_inbox_id: form.chatwoot_inbox_id ? parseInt(form.chatwoot_inbox_id, 10) : 0,
      chatwoot_webhook_secret: form.chatwoot_webhook_secret.trim(),

      google_calendar_enabled: form.google_calendar_enabled,
      google_calendar_id: form.google_calendar_id.trim(),
      google_calendar_token: form.google_calendar_token.trim(),
      google_client_id: form.google_client_id.trim(),
      google_client_secret: form.google_client_secret.trim(),

      clerk_enabled: form.clerk_enabled,
      clerk_publishable_key: form.clerk_publishable_key.trim(),
      clerk_secret_key: form.clerk_secret_key.trim(),
    }

    try {
      await api.patch(`/t/${tenant}/admin/settings`, payload)
      setMsg({ error: '', success: 'Omnichannel & integration settings saved successfully!' })
      setTimeout(() => setMsg((m) => ({ ...m, success: '' })), 4000)
    } catch (err) {
      setMsg({
        error: err?.response?.data?.message || err?.message || 'Failed to update settings',
        success: '',
      })
    } finally {
      setSaving(false)
    }
  }

  const handleTestChatwoot = async () => {
    setTestingChatwoot(true)
    setTestResult(null)
    try {
      const res = await api.post(`/t/${tenant}/admin/integrations/chatwoot/test`)
      setTestResult({ ok: true, data: res.data })
    } catch (err) {
      setTestResult({
        ok: false,
        error: err?.response?.data?.message || 'Connection failed. Ensure settings are saved first.',
      })
    } finally {
      setTestingChatwoot(false)
    }
  }

  const handleResumeAI = async (sessionID) => {
    try {
      await api.post(`/t/${tenant}/admin/integrations/chatwoot/sessions/${sessionID}/resume-ai`)
      fetchSettingsAndMonitorData()
    } catch (err) {
      setMsg({ error: 'Failed to resume AI session', success: '' })
    }
  }

  const handleAddBranch = async () => {
    if (!newBranch.name.trim()) return
    try {
      await api.post(`/t/${tenant}/admin/branches`, newBranch)
      setNewBranch({ name: '', address: '', phone: '' })
      fetchSettingsAndMonitorData()
    } catch (err) {
      setMsg({ error: err?.response?.data?.message || 'Failed to add branch', success: '' })
    }
  }

  const handleDeleteBranch = async (id) => {
    try {
      await api.delete(`/t/${tenant}/admin/branches/${id}`)
      fetchSettingsAndMonitorData()
    } catch (err) {
      setMsg({ error: 'Failed to delete branch', success: '' })
    }
  }

  // Derive webhook URLs for Chatwoot
  const apiBase =
    process.env.NEXT_PUBLIC_API_URL ||
    (typeof window !== 'undefined' ? `${window.location.protocol}//${window.location.host}` : '')
  const cleanBase = apiBase.replace(/\/+$/, '')
  const webhookUrl = `${cleanBase}/t/${tenant}/webhooks/chatwoot`
  const globalWebhookUrl = `${cleanBase}/webhooks/chatwoot`

  const handleCopy = (val, isGlobal = false) => {
    if (typeof navigator !== 'undefined' && navigator.clipboard) {
      navigator.clipboard.writeText(val)
      if (isGlobal) {
        setCopiedGlobal(true)
        setTimeout(() => setCopiedGlobal(false), 2000)
      } else {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      }
    }
  }

  if (loading) {
    return (
      <div className="bg-white border rounded-xl p-8 text-center text-gray-500 shadow-sm">
        <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mb-2"></div>
        <p className="text-sm">Loading integration settings...</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="bg-white border rounded-xl p-6 shadow-sm">
        <h1 className="text-xl font-bold text-gray-900">Omnichannel Integrations &amp; Provider Configuration</h1>
        <p className="text-sm text-gray-500 mt-1">
          Configure your WhatsApp Business inbox via Chatwoot, AI receptionist rules, hospital branches, Google Calendar sync, and Clerk SSO.
        </p>
      </div>

      {msg.error && (
        <div className="p-4 bg-red-50 border border-red-200 text-red-700 text-sm rounded-xl">
          {msg.error}
        </div>
      )}
      {msg.success && (
        <div className="p-4 bg-emerald-50 border border-emerald-200 text-emerald-800 text-sm rounded-xl">
          {msg.success}
        </div>
      )}

      <form onSubmit={handleSave} className="space-y-6">
        {/* SECTION 1: WHATSAPP VIA CHATWOOT */}
        <div className="bg-white border rounded-xl p-6 shadow-sm space-y-5">
          <div className="flex items-center justify-between border-b pb-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-emerald-100 text-emerald-700 flex items-center justify-center font-bold text-lg">
                💬
              </div>
              <div>
                <h2 className="text-base font-bold text-gray-900">WhatsApp Omnichannel (via Chatwoot)</h2>
                <p className="text-xs text-gray-500">
                  Map your dedicated WhatsApp Business inbox to this healthcare provider for isolated multi-tenant bookings.
                </p>
              </div>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={form.chatwoot_enabled}
                onChange={(e) => setForm({ ...form, chatwoot_enabled: e.target.checked })}
                className="sr-only peer"
              />
              <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-emerald-600"></div>
            </label>
          </div>

          <div className="space-y-4">
            {/* Webhook URL Helper */}
            <div className="bg-slate-50 border rounded-xl p-4 space-y-3">
              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1">
                  Provider Webhook URL (Dedicated Tenant Endpoint)
                </label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    readOnly
                    value={webhookUrl}
                    className="flex-1 border rounded-lg px-3 py-2 text-xs font-mono bg-white text-slate-800"
                  />
                  <button
                    type="button"
                    onClick={() => handleCopy(webhookUrl, false)}
                    className="px-3 py-2 bg-slate-200 hover:bg-slate-300 text-slate-800 text-xs font-medium rounded-lg transition"
                  >
                    {copied ? 'Copied! ✓' : 'Copy'}
                  </button>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1">
                  Global Inbox Router Webhook URL (Routes Multiple Providers by Inbox ID)
                </label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    readOnly
                    value={globalWebhookUrl}
                    className="flex-1 border rounded-lg px-3 py-2 text-xs font-mono bg-white text-slate-800"
                  />
                  <button
                    type="button"
                    onClick={() => handleCopy(globalWebhookUrl, true)}
                    className="px-3 py-2 bg-slate-200 hover:bg-slate-300 text-slate-800 text-xs font-medium rounded-lg transition"
                  >
                    {copiedGlobal ? 'Copied! ✓' : 'Copy'}
                  </button>
                </div>
              </div>

              <p className="text-[11px] text-slate-500">
                Subscribe Chatwoot webhooks to <code>message_created</code> and <code>conversation_status_changed</code>.
              </p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Chatwoot Base URL
                </label>
                <input
                  type="url"
                  placeholder="https://app.chatwoot.com"
                  className="w-full border rounded-lg p-2.5 text-sm bg-white"
                  value={form.chatwoot_base_url}
                  onChange={(e) => setForm({ ...form, chatwoot_base_url: e.target.value })}
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  WhatsApp Business Number
                </label>
                <input
                  type="text"
                  placeholder="e.g. +254712345678"
                  className="w-full border rounded-lg p-2.5 text-sm bg-white"
                  value={form.whatsapp_number}
                  onChange={(e) => setForm({ ...form, whatsapp_number: e.target.value })}
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Chatwoot Account ID
                </label>
                <input
                  type="number"
                  placeholder="e.g. 1"
                  className="w-full border rounded-lg p-2.5 text-sm bg-white"
                  value={form.chatwoot_account_id}
                  onChange={(e) => setForm({ ...form, chatwoot_account_id: e.target.value })}
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Mapped WhatsApp Inbox ID
                </label>
                <input
                  type="number"
                  placeholder="e.g. 1"
                  className="w-full border rounded-lg p-2.5 text-sm bg-white"
                  value={form.chatwoot_inbox_id}
                  onChange={(e) => setForm({ ...form, chatwoot_inbox_id: e.target.value })}
                />
              </div>

              <div>
                <div className="flex justify-between items-center mb-1">
                  <label className="text-xs font-semibold text-gray-700">API Access Token</label>
                  <button
                    type="button"
                    onClick={() => setShowTokens((s) => ({ ...s, chatwoot: !s.chatwoot }))}
                    className="text-[11px] text-blue-600 hover:underline"
                  >
                    {showTokens.chatwoot ? 'Hide' : 'Show'}
                  </button>
                </div>
                <input
                  type={showTokens.chatwoot ? 'text' : 'password'}
                  placeholder="Paste Chatwoot User / Bot Token"
                  className="w-full border rounded-lg p-2.5 text-sm font-mono bg-white"
                  value={form.chatwoot_api_token}
                  onChange={(e) => setForm({ ...form, chatwoot_api_token: e.target.value })}
                />
              </div>

              <div>
                <div className="flex justify-between items-center mb-1">
                  <label className="text-xs font-semibold text-gray-700">Webhook Signing Secret (Optional)</label>
                  <button
                    type="button"
                    onClick={() => setShowTokens((s) => ({ ...s, chatwootSecret: !s.chatwootSecret }))}
                    className="text-[11px] text-blue-600 hover:underline"
                  >
                    {showTokens.chatwootSecret ? 'Hide' : 'Show'}
                  </button>
                </div>
                <input
                  type={showTokens.chatwootSecret ? 'text' : 'password'}
                  placeholder="HMAC Secret or Webhook Token"
                  className="w-full border rounded-lg p-2.5 text-sm font-mono bg-white"
                  value={form.chatwoot_webhook_secret}
                  onChange={(e) => setForm({ ...form, chatwoot_webhook_secret: e.target.value })}
                />
              </div>
            </div>

            {/* Test Connectivity Button */}
            <div className="flex items-center gap-3 pt-2">
              <button
                type="button"
                onClick={handleTestChatwoot}
                disabled={testingChatwoot}
                className="px-4 py-2 bg-emerald-50 hover:bg-emerald-100 text-emerald-800 border border-emerald-200 rounded-lg text-xs font-semibold transition"
              >
                {testingChatwoot ? 'Testing Connectivity...' : '🔌 Test Chatwoot & Inbox Connection'}
              </button>
              {testResult && (
                <span className={`text-xs font-medium ${testResult.ok ? 'text-emerald-700' : 'text-red-600'}`}>
                  {testResult.ok
                    ? `Connected! Account #${testResult.data.account_id} · Inbox Verified: ${
                        testResult.data.inbox_verified ? `Yes (${testResult.data.matched_inbox_name})` : 'No matching inbox found'
                      }`
                    : testResult.error}
                </span>
              )}
            </div>
          </div>
        </div>

        {/* SECTION 2: AI RECEPTIONIST & PROVIDER PROFILE */}
        <div className="bg-white border rounded-xl p-6 shadow-sm space-y-5">
          <div className="border-b pb-4">
            <h2 className="text-base font-bold text-gray-900">AI Receptionist &amp; Provider Details</h2>
            <p className="text-xs text-gray-500">
              Control AI behavior, hospital address, business hours, policies, and marketplace search visibility.
            </p>
          </div>

          <div className="flex flex-wrap gap-6 bg-slate-50 p-4 rounded-xl border">
            <label className="flex items-center gap-2.5 cursor-pointer">
              <input
                type="checkbox"
                checked={form.ai_receptionist_enabled}
                onChange={(e) => setForm({ ...form, ai_receptionist_enabled: e.target.checked })}
                className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              <div>
                <span className="text-sm font-semibold text-gray-900 block">Enable AI Receptionist</span>
                <span className="text-[11px] text-gray-500">Auto-handle incoming WhatsApp appointment requests</span>
              </div>
            </label>

            <label className="flex items-center gap-2.5 cursor-pointer">
              <input
                type="checkbox"
                checked={form.marketplace_enabled}
                onChange={(e) => setForm({ ...form, marketplace_enabled: e.target.checked })}
                className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              <div>
                <span className="text-sm font-semibold text-gray-900 block">Marketplace Cross-Hospital Search</span>
                <span className="text-[11px] text-gray-500">Allow participating provider discovery &amp; cross-referrals</span>
              </div>
            </label>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Provider Type</label>
              <select
                className="w-full border rounded-lg p-2.5 text-sm bg-white"
                value={form.provider_type}
                onChange={(e) => setForm({ ...form, provider_type: e.target.value })}
              >
                <option value="clinic">Outpatient Clinic</option>
                <option value="hospital">Hospital</option>
                <option value="medical_center">Medical Centre</option>
                <option value="specialist_practice">Specialist Practice</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Main Phone Line</label>
              <input
                type="text"
                placeholder="+254..."
                className="w-full border rounded-lg p-2.5 text-sm bg-white"
                value={form.phone}
                onChange={(e) => setForm({ ...form, phone: e.target.value })}
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Physical Address &amp; Directions</label>
              <input
                type="text"
                placeholder="e.g. 3rd Ngong Ave, Nairobi"
                className="w-full border rounded-lg p-2.5 text-sm bg-white"
                value={form.address}
                onChange={(e) => setForm({ ...form, address: e.target.value })}
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Operating / Business Hours</label>
              <input
                type="text"
                placeholder="Mon-Fri 08:00-18:00, Sat 09:00-13:00"
                className="w-full border rounded-lg p-2.5 text-sm bg-white"
                value={form.business_hours}
                onChange={(e) => setForm({ ...form, business_hours: e.target.value })}
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Cancellation Policy</label>
              <input
                type="text"
                placeholder="e.g. Free cancellation up to 4 hours prior"
                className="w-full border rounded-lg p-2.5 text-sm bg-white"
                value={form.cancellation_policy}
                onChange={(e) => setForm({ ...form, cancellation_policy: e.target.value })}
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">Human Handover Keywords</label>
              <input
                type="text"
                placeholder="human,agent,receptionist,speak to someone"
                className="w-full border rounded-lg p-2.5 text-sm bg-white"
                value={form.handoff_keywords}
                onChange={(e) => setForm({ ...form, handoff_keywords: e.target.value })}
              />
            </div>
          </div>
        </div>

        {/* SECTION 3: GOOGLE CALENDAR 2-WAY SYNC */}
        <div className="bg-white border rounded-xl p-6 shadow-sm space-y-5">
          <div className="flex items-center justify-between border-b pb-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-blue-100 text-blue-700 flex items-center justify-center font-bold text-lg">
                📅
              </div>
              <div>
                <h2 className="text-base font-bold text-gray-900">Google Calendar Two-Way Sync</h2>
                <p className="text-xs text-gray-500">
                  Sync confirmed appointments directly to Google Calendar and automatically block busy times.
                </p>
              </div>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={form.google_calendar_enabled}
                onChange={(e) => setForm({ ...form, google_calendar_enabled: e.target.checked })}
                className="sr-only peer"
              />
              <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
            </label>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Calendar ID (e.g. &apos;primary&apos; or doctor@clinic.com)
              </label>
              <input
                type="text"
                placeholder="primary"
                className="w-full border rounded-lg p-2.5 text-sm bg-white"
                value={form.google_calendar_id}
                onChange={(e) => setForm({ ...form, google_calendar_id: e.target.value })}
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Google Client ID
              </label>
              <input
                type="text"
                placeholder="apps.googleusercontent.com Client ID"
                className="w-full border rounded-lg p-2.5 text-sm bg-white"
                value={form.google_client_id}
                onChange={(e) => setForm({ ...form, google_client_id: e.target.value })}
              />
            </div>

            <div>
              <div className="flex justify-between items-center mb-1">
                <label className="text-xs font-semibold text-gray-700">Google Client Secret</label>
                <button
                  type="button"
                  onClick={() => setShowTokens((s) => ({ ...s, gcalendar: !s.gcalendar }))}
                  className="text-[11px] text-blue-600 hover:underline"
                >
                  {showTokens.gcalendar ? 'Hide' : 'Show'}
                </button>
              </div>
              <input
                type={showTokens.gcalendar ? 'text' : 'password'}
                placeholder="Client Secret"
                className="w-full border rounded-lg p-2.5 text-sm font-mono bg-white"
                value={form.google_client_secret}
                onChange={(e) => setForm({ ...form, google_client_secret: e.target.value })}
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                OAuth Refresh Token
              </label>
              <input
                type="password"
                placeholder="Google OAuth Refresh Token"
                className="w-full border rounded-lg p-2.5 text-sm font-mono bg-white"
                value={form.google_calendar_token}
                onChange={(e) => setForm({ ...form, google_calendar_token: e.target.value })}
              />
            </div>
          </div>
        </div>

        {/* SECTION 4: CLERK SINGLE SIGN-ON */}
        <div className="bg-white border rounded-xl p-6 shadow-sm space-y-5">
          <div className="flex items-center justify-between border-b pb-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-purple-100 text-purple-700 flex items-center justify-center font-bold text-lg">
                🔐
              </div>
              <div>
                <h2 className="text-base font-bold text-gray-900">Clerk Single Sign-On (SSO)</h2>
                <p className="text-xs text-gray-500">
                  Authenticate practice administrators, doctors, and staff with Google via Clerk.
                </p>
              </div>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={form.clerk_enabled}
                onChange={(e) => setForm({ ...form, clerk_enabled: e.target.checked })}
                className="sr-only peer"
              />
              <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-purple-600"></div>
            </label>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 mb-1">
                Clerk Publishable Key
              </label>
              <input
                type="text"
                placeholder="pk_test_... or pk_live_..."
                className="w-full border rounded-lg p-2.5 text-sm font-mono bg-white"
                value={form.clerk_publishable_key}
                onChange={(e) => setForm({ ...form, clerk_publishable_key: e.target.value })}
              />
            </div>

            <div>
              <div className="flex justify-between items-center mb-1">
                <label className="text-xs font-semibold text-gray-700">Clerk Secret Key</label>
                <button
                  type="button"
                  onClick={() => setShowTokens((s) => ({ ...s, clerk: !s.clerk }))}
                  className="text-[11px] text-blue-600 hover:underline"
                >
                  {showTokens.clerk ? 'Hide' : 'Show'}
                </button>
              </div>
              <input
                type={showTokens.clerk ? 'text' : 'password'}
                placeholder="sk_test_... or sk_live_..."
                className="w-full border rounded-lg p-2.5 text-sm font-mono bg-white"
                value={form.clerk_secret_key}
                onChange={(e) => setForm({ ...form, clerk_secret_key: e.target.value })}
              />
            </div>
          </div>
        </div>

        {/* Submit */}
        <div className="flex justify-end pt-2">
          <button
            type="submit"
            disabled={saving}
            className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-xl text-sm font-semibold transition shadow-sm disabled:opacity-60"
          >
            {saving ? 'Saving...' : 'Save All Settings'}
          </button>
        </div>
      </form>

      {/* HOSPITAL BRANCHES / LOCATIONS */}
      <div className="bg-white border rounded-xl p-6 shadow-sm space-y-4">
        <div className="border-b pb-3">
          <h2 className="text-base font-bold text-gray-900">Hospital Branches &amp; Locations</h2>
          <p className="text-xs text-gray-500">Add clinic branches so WhatsApp patients can filter doctors and get directions.</p>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
          <input
            type="text"
            placeholder="Branch Name (e.g. Westlands)"
            className="border rounded-lg p-2 text-sm"
            value={newBranch.name}
            onChange={(e) => setNewBranch({ ...newBranch, name: e.target.value })}
          />
          <input
            type="text"
            placeholder="Address"
            className="border rounded-lg p-2 text-sm"
            value={newBranch.address}
            onChange={(e) => setNewBranch({ ...newBranch, address: e.target.value })}
          />
          <input
            type="text"
            placeholder="Branch Phone"
            className="border rounded-lg p-2 text-sm"
            value={newBranch.phone}
            onChange={(e) => setNewBranch({ ...newBranch, phone: e.target.value })}
          />
          <button
            type="button"
            onClick={handleAddBranch}
            className="bg-slate-800 hover:bg-slate-900 text-white rounded-lg text-xs font-semibold px-4 py-2 transition"
          >
            + Add Branch
          </button>
        </div>
        {branches.length > 0 && (
          <div className="divide-y border rounded-lg">
            {branches.map((b) => (
              <div key={b.id} className="p-3 flex items-center justify-between text-xs">
                <div>
                  <span className="font-bold text-gray-900">{b.name}</span>
                  {b.address && <span className="text-gray-500 ml-2">· {b.address}</span>}
                  {b.phone && <span className="text-gray-500 ml-2">· {b.phone}</span>}
                </div>
                <button
                  type="button"
                  onClick={() => handleDeleteBranch(b.id)}
                  className="text-rose-600 hover:underline font-medium"
                >
                  Remove
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* LIVE WHATSAPP SESSIONS & HANDOVER MONITOR */}
      <div className="bg-white border rounded-xl p-6 shadow-sm space-y-4">
        <div className="flex items-center justify-between border-b pb-3">
          <div>
            <h2 className="text-base font-bold text-gray-900">WhatsApp Conversation Sessions Monitor</h2>
            <p className="text-xs text-gray-500">Persistent AI conversations and human takeover status.</p>
          </div>
          <button
            type="button"
            onClick={fetchSettingsAndMonitorData}
            className="text-xs text-blue-600 hover:underline font-medium"
          >
            Refresh
          </button>
        </div>

        {sessions.length === 0 ? (
          <p className="text-xs text-gray-500 text-center py-6">No WhatsApp conversations recorded yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead>
                <tr className="border-b text-gray-500 uppercase">
                  <th className="py-2 px-3">Patient</th>
                  <th className="py-2 px-3">Conv ID</th>
                  <th className="py-2 px-3">Workflow State</th>
                  <th className="py-2 px-3">Status</th>
                  <th className="py-2 px-3 text-right">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {sessions.map((s) => (
                  <tr key={s.id} className="hover:bg-slate-50">
                    <td className="py-2.5 px-3">
                      <div className="font-semibold text-gray-900">{s.patient_name || 'Unknown'}</div>
                      <div className="text-gray-500">{s.patient_phone}</div>
                    </td>
                    <td className="py-2.5 px-3 font-mono text-gray-700">#{s.chatwoot_conversation_id}</td>
                    <td className="py-2.5 px-3">{s.workflow_state || 'initial'}</td>
                    <td className="py-2.5 px-3">
                      {s.human_handover ? (
                        <span className="px-2 py-0.5 rounded-full bg-amber-100 text-amber-800 font-semibold">
                          Human Takeover
                        </span>
                      ) : (
                        <span className="px-2 py-0.5 rounded-full bg-emerald-100 text-emerald-800 font-semibold">
                          AI Active
                        </span>
                      )}
                    </td>
                    <td className="py-2.5 px-3 text-right">
                      {s.human_handover && (
                        <button
                          type="button"
                          onClick={() => handleResumeAI(s.session_id)}
                          className="text-blue-600 hover:underline font-semibold"
                        >
                          Resume AI Bot
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
