"use client"
import { useEffect, useState, useCallback } from 'react'
import api from '../lib/api'

export default function IntegrationsSettings({ tenant = 'default' }) {
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [copied, setCopied] = useState(false)
  const [msg, setMsg] = useState({ error: '', success: '' })

  const [form, setForm] = useState({
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
    gcalendar: false,
    clerk: false,
  })

  const fetchSettings = useCallback(async () => {
    setLoading(true)
    setMsg({ error: '', success: '' })
    try {
      const res = await api.get(`/t/${tenant}/admin/settings`)
      const d = res.data || {}
      setForm({
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
    fetchSettings()
  }, [fetchSettings])

  const handleSave = async (e) => {
    e.preventDefault()
    setSaving(true)
    setMsg({ error: '', success: '' })

    const payload = {
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
      setMsg({ error: '', success: 'Integration settings saved successfully!' })
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

  // Derive webhook URL for Chatwoot
  const apiBase =
    process.env.NEXT_PUBLIC_API_URL ||
    (typeof window !== 'undefined' ? `${window.location.protocol}//${window.location.host}` : '')
  const webhookUrl = `${apiBase.replace(/\/+$/, '')}/t/${tenant}/webhooks/chatwoot`

  const handleCopyWebhook = () => {
    if (typeof navigator !== 'undefined' && navigator.clipboard) {
      navigator.clipboard.writeText(webhookUrl)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
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
        <h1 className="text-xl font-bold text-gray-900">Integrations &amp; Channels</h1>
        <p className="text-sm text-gray-500 mt-1">
          Connect your Booka clinic with WhatsApp via Chatwoot, sync appointments with Google Calendar, and enable Clerk Single Sign-On.
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
                <h2 className="text-base font-bold text-gray-900">WhatsApp Integration (via Chatwoot)</h2>
                <p className="text-xs text-gray-500">
                  Allow patients to book and reschedule medical appointments directly through WhatsApp with AI and receptionist handoff.
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
            <div className="bg-slate-50 border rounded-xl p-4 space-y-2">
              <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider">
                Chatwoot Webhook URL (Paste this into Chatwoot Webhooks)
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
                  onClick={handleCopyWebhook}
                  className="px-3 py-2 bg-slate-200 hover:bg-slate-300 text-slate-800 text-xs font-medium rounded-lg transition"
                >
                  {copied ? 'Copied! ✓' : 'Copy URL'}
                </button>
              </div>
              <p className="text-[11px] text-slate-500">
                In Chatwoot, navigate to <strong>Settings &gt; Integrations &gt; Webhooks &gt; Add New Webhook</strong>, paste this URL, and subscribe to <code>message_created</code>.
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
                  Account ID
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
                  WhatsApp Inbox ID
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
            </div>
          </div>
        </div>

        {/* SECTION 2: GOOGLE CALENDAR 2-WAY SYNC */}
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

        {/* SECTION 3: CLERK SINGLE SIGN-ON */}
        <div className="bg-white border rounded-xl p-6 shadow-sm space-y-5">
          <div className="flex items-center justify-between border-b pb-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-purple-100 text-purple-700 flex items-center justify-center font-bold text-lg">
                🔐
              </div>
              <div>
                <h2 className="text-base font-bold text-gray-900">Clerk Single Sign-On (SSO)</h2>
                <p className="text-xs text-gray-500">
                  Authenticate practice administrators, doctors, and staff with Google, Apple, Microsoft, or SAML via Clerk.
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
            {saving ? 'Saving...' : 'Save Integrations'}
          </button>
        </div>
      </form>
    </div>
  )
}
