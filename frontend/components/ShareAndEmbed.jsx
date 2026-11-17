"use client"
import React, { useState, useEffect, useRef, useCallback } from 'react'
import QRCode from 'qrcode'
import api from '../lib/api'

export function validateOriginClient(raw, isDev = false) {
  const orig = String(raw || '').trim().toLowerCase()
  if (!orig) {
    return { valid: false, error: 'origin cannot be empty' }
  }

  if (orig.endsWith('/') || orig.includes('?') || orig.includes('#')) {
    return { valid: false, error: 'origin must not contain paths, queries, fragments or trailing slashes' }
  }

  if (orig.startsWith('http://localhost')) {
    if (!isDev) {
      return { valid: false, error: 'http://localhost is only allowed in development environment' }
    }
    const rest = orig.slice('http://localhost'.length)
    if (rest) {
      if (!rest.startsWith(':')) {
        return { valid: false, error: 'invalid localhost origin' }
      }
      const port = Number(rest.slice(1))
      if (isNaN(port) || port <= 0 || port > 65535) {
        return { valid: false, error: 'invalid port number on localhost' }
      }
    }
    return { valid: true, origin: orig }
  }

  if (!orig.startsWith('https://')) {
    return { valid: false, error: 'origin must use https://' }
  }

  const hostPart = orig.slice('https://'.length)
  if (!hostPart) {
    return { valid: false, error: 'missing host in origin' }
  }

  if (hostPart.includes('/')) {
    return { valid: false, error: 'origin must not contain a path' }
  }

  let host = hostPart
  let portStr = ''
  const lastColon = hostPart.lastIndexOf(':')
  if (lastColon !== -1) {
    host = hostPart.slice(0, lastColon)
    portStr = hostPart.slice(lastColon + 1)
    if (!portStr) {
      return { valid: false, error: 'invalid port specification' }
    }
    const port = Number(portStr)
    if (isNaN(port) || port <= 0 || port > 65535) {
      return { valid: false, error: 'invalid port number' }
    }
  }

  const isValidDomainLabel = (label) => {
    if (!label || label.length > 63) return false
    for (let i = 0; i < label.length; i++) {
      const c = label.charCodeAt(i)
      const isAlnum = (c >= 97 && c <= 122) || (c >= 48 && c <= 57)
      if (isAlnum) continue
      if (label[i] === '-' && i > 0 && i < label.length - 1) continue
      return false
    }
    return true
  }

  if (host.includes('*')) {
    if (!host.startsWith('*.')) {
      return { valid: false, error: "wildcard '*' is only allowed as the leading label, e.g. https://*.example.com" }
    }
    if ((host.match(/\*/g) || []).length > 1) {
      return { valid: false, error: 'multiple wildcards are not allowed' }
    }
    const remaining = host.slice(2)
    if (!remaining || remaining.includes('*') || remaining.startsWith('.') || remaining.endsWith('.')) {
      return { valid: false, error: 'invalid wildcard host pattern' }
    }
    const labels = remaining.split('.')
    for (const label of labels) {
      if (!isValidDomainLabel(label)) {
        return { valid: false, error: `invalid domain label '${label}'` }
      }
    }
  } else {
    const labels = host.split('.')
    for (const label of labels) {
      if (!isValidDomainLabel(label)) {
        return { valid: false, error: `invalid domain label '${label}'` }
      }
    }
  }

  const normalized = portStr ? `https://${host}:${portStr}` : `https://${host}`
  return { valid: true, origin: normalized }
}

export function escapeHtml(str) {
  if (!str) return ''
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

export default function ShareAndEmbed({ tenant = 'default' }) {
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')
  const [successMsg, setSuccessMsg] = useState('')

  // Settings
  const [bookingEnabled, setBookingEnabled] = useState(true)
  const [allowedOrigins, setAllowedOrigins] = useState([])
  const [privacyUrl, setPrivacyUrl] = useState('')
  const [clinicName, setClinicName] = useState('')

  // Domain input & validation
  const [newDomain, setNewDomain] = useState('')
  const [domainError, setDomainError] = useState('')

  // Query parameter builder
  const [paramSource, setParamSource] = useState('')
  const [paramDoctor, setParamDoctor] = useState('')
  const [paramReason, setParamReason] = useState('')

  // Copy feedback states
  const [copiedDirect, setCopiedDirect] = useState(false)
  const [copiedIframe, setCopiedIframe] = useState(false)
  const [copiedWidget, setCopiedWidget] = useState(false)

  // QR canvas
  const canvasRef = useRef(null)
  const [qrDataUrl, setQrDataUrl] = useState('')

  // Base URL
  const [baseUrl, setBaseUrl] = useState('')
  useEffect(() => {
    if (typeof window !== 'undefined') {
      setBaseUrl(window.location.origin)
    }
  }, [])

  // Load settings
  const loadSettings = useCallback(async () => {
    setLoading(true)
    setErrorMsg('')
    try {
      const res = await api.get(`/t/${tenant}/admin/settings`)
      const d = res.data || {}
      setBookingEnabled(d.booking_link_enabled ?? true)
      setAllowedOrigins(Array.isArray(d.allowed_embed_origins) ? d.allowed_embed_origins : [])
      setPrivacyUrl(d.privacy_url || '')
    } catch (e) {
      setErrorMsg(e?.response?.data?.message || 'Failed to load clinic settings')
    } finally {
      setLoading(false)
    }
  }, [tenant])

  // Load clinic public config for name
  useEffect(() => {
    api.get(`/t/${tenant}/public-config`)
      .then((res) => {
        if (res.data?.name) setClinicName(res.data.name)
      })
      .catch(() => {})
  }, [tenant])

  useEffect(() => {
    loadSettings()
  }, [loadSettings])

  // Compute query string from active sample params
  const queryString = (() => {
    const p = new URLSearchParams()
    if (paramSource.trim()) p.set('source', paramSource.trim())
    if (paramDoctor.trim()) p.set('doctor', paramDoctor.trim())
    if (paramReason.trim()) p.set('reason', paramReason.trim())
    const s = p.toString()
    return s ? `?${s}` : ''
  })()

  // Computed URLs
  const originUrl = baseUrl || 'https://example.com'
  const directLink = `${originUrl}/book/${encodeURIComponent(tenant)}${queryString}`
  const embedLink = `${originUrl}/embed/${encodeURIComponent(tenant)}${queryString}`
  const widgetScriptUrl = `${originUrl}/embed.js`

  // Render QR Code
  useEffect(() => {
    if (!canvasRef.current || !directLink) return
    try {
      QRCode.toCanvas(
        canvasRef.current,
        directLink,
        {
          width: 180,
          margin: 2,
          color: { dark: '#0f172a', light: '#ffffff' },
        },
        (err) => {
          if (!err && canvasRef.current) {
            try {
              setQrDataUrl(canvasRef.current.toDataURL('image/png'))
            } catch {
              // canvas tainted or jsdom
            }
          }
        }
      )
    } catch {
      // jsdom environment fallback
    }
  }, [directLink])

  // Snippets
  const escapedEmbedUrl = escapeHtml(embedLink)
  const escapedClinicTitle = escapeHtml(`${clinicName || tenant} - Online Appointment Booking`)
  const iframeSnippet = `<iframe src="${escapedEmbedUrl}" title="${escapedClinicTitle}" width="100%" height="640" loading="lazy" style="border:none;border-radius:12px;"></iframe>`

  const widgetQueryParam = queryString ? ` data-query="${escapeHtml(queryString)}"` : ''
  const widgetSnippet = `<script src="${escapeHtml(widgetScriptUrl)}" data-clinic="${escapeHtml(tenant)}"${widgetQueryParam}></script>`

  // Copy handler
  const handleCopy = async (text, setCopied) => {
    try {
      if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text)
      }
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Fallback
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  // Domain add
  const handleAddDomain = () => {
    setDomainError('')
    if (!newDomain.trim()) {
      setDomainError('origin cannot be empty')
      return
    }

    if (allowedOrigins.length >= 10) {
      setDomainError('maximum 10 embed origins allowed')
      return
    }

    const isDev = process.env.NODE_ENV === 'development' || (typeof window !== 'undefined' && window.location.hostname === 'localhost')
    const val = validateOriginClient(newDomain, isDev)
    if (!val.valid) {
      setDomainError(val.error)
      return
    }

    if (allowedOrigins.includes(val.origin)) {
      setDomainError('Domain is already in the allowed list')
      return
    }

    setAllowedOrigins([...allowedOrigins, val.origin])
    setNewDomain('')
  }

  // Domain remove
  const handleRemoveDomain = (domainToRemove) => {
    setAllowedOrigins(allowedOrigins.filter((d) => d !== domainToRemove))
  }

  // Save Settings
  const handleSave = async (e) => {
    if (e) e.preventDefault()
    setSaving(true)
    setErrorMsg('')
    setSuccessMsg('')

    if (privacyUrl.trim()) {
      const p = privacyUrl.trim()
      if (!p.startsWith('https://')) {
        setErrorMsg('privacy_url must be a valid https URL')
        setSaving(false)
        return
      }
      if (p.length > 300) {
        setErrorMsg('privacy_url must be at most 300 characters')
        setSaving(false)
        return
      }
    }

    try {
      await api.patch(`/t/${tenant}/admin/settings`, {
        booking_link_enabled: bookingEnabled,
        allowed_embed_origins: allowedOrigins,
        privacy_url: privacyUrl.trim(),
      })
      setSuccessMsg('Settings saved successfully!')
      setTimeout(() => setSuccessMsg(''), 4000)
    } catch (err) {
      setErrorMsg(err?.response?.data?.message || 'Failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="bg-white border rounded-xl p-8 text-center text-slate-500 font-medium text-sm">
        Loading Share & Embed configurations...
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header and Toggle */}
      <div className="bg-white border rounded-xl p-5 shadow-sm space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b pb-4">
          <div>
            <h2 className="text-lg font-bold text-slate-900">Share & Embed Clinic Booking</h2>
            <p className="text-xs text-slate-500">
              Provide patients direct access to appointment booking via link, QR code, iframe embed, or floating widget.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <label className="flex items-center gap-2 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={bookingEnabled}
                onChange={(e) => setBookingEnabled(e.target.checked)}
                className="w-4 h-4 text-blue-600 rounded focus:ring-blue-500"
              />
              <span className="text-sm font-semibold text-slate-800">
                Online Booking {bookingEnabled ? 'Enabled' : 'Disabled'}
              </span>
            </label>
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white font-medium text-xs rounded-lg transition disabled:opacity-60 shadow-sm"
            >
              {saving ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </div>

        {errorMsg && (
          <div className="p-3 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg">
            {errorMsg}
          </div>
        )}
        {successMsg && (
          <div className="p-3 bg-green-50 border border-green-200 text-green-700 text-sm rounded-lg">
            {successMsg}
          </div>
        )}

        {/* Privacy URL */}
        <div className="pt-1">
          <label className="block text-xs font-semibold text-slate-700 mb-1">
            Privacy Policy URL (Optional, must be https://, max 300 characters)
          </label>
          <input
            type="url"
            value={privacyUrl}
            maxLength={300}
            onChange={(e) => setPrivacyUrl(e.target.value)}
            placeholder="https://yourclinic.com/privacy"
            className="w-full max-w-xl border rounded-lg p-2 text-sm bg-slate-50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <span className="text-[11px] text-slate-500 block mt-1">
            Displayed on public booking pages linked from the AI data processing disclosure.
          </span>
        </div>
      </div>

      {/* URL Parameters / Source Campaign Builder */}
      <div className="bg-white border rounded-xl p-5 shadow-sm space-y-3">
        <h3 className="text-sm font-bold text-slate-900">Custom Campaign & Pre-fill Options</h3>
        <p className="text-xs text-slate-500">
          Append query parameters to track appointment attribution or pre-select a specific doctor or visit reason.
        </p>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1">
              Traffic Source (?source=)
            </label>
            <input
              type="text"
              placeholder="e.g. instagram, newsletter"
              value={paramSource}
              onChange={(e) => setParamSource(e.target.value)}
              className="w-full border rounded-lg p-2 text-xs bg-slate-50 focus:bg-white"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1">
              Doctor ID (?doctor=)
            </label>
            <input
              type="number"
              placeholder="e.g. 1"
              value={paramDoctor}
              onChange={(e) => setParamDoctor(e.target.value)}
              className="w-full border rounded-lg p-2 text-xs bg-slate-50 focus:bg-white"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1">
              Reason ID (?reason=)
            </label>
            <input
              type="number"
              placeholder="e.g. 2"
              value={paramReason}
              onChange={(e) => setParamReason(e.target.value)}
              className="w-full border rounded-lg p-2 text-xs bg-slate-50 focus:bg-white"
            />
          </div>
        </div>
      </div>

      {/* Direct Booking Link & QR Code */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 bg-white border rounded-xl p-5 shadow-sm space-y-4 flex flex-col justify-between">
          <div className="space-y-2">
            <h3 className="text-sm font-bold text-slate-900">Direct Booking Link</h3>
            <p className="text-xs text-slate-500">
              Share this full-page booking link directly with your patients via SMS, email, or social media.
            </p>
            <div className="flex items-center gap-2 pt-2">
              <input
                type="text"
                readOnly
                value={directLink}
                className="flex-1 bg-slate-50 border rounded-lg p-2.5 text-xs text-slate-700 font-mono select-all focus:outline-none"
              />
              <button
                onClick={() => handleCopy(directLink, setCopiedDirect)}
                className="px-4 py-2.5 bg-slate-900 hover:bg-slate-800 text-white font-medium text-xs rounded-lg transition"
              >
                {copiedDirect ? '✓ Copied!' : 'Copy Link'}
              </button>
            </div>
          </div>

          <div className="pt-4 border-t text-xs text-slate-500">
            Patients visiting this link will access your dedicated full-page AI booking assistant.
          </div>
        </div>

        {/* QR Code */}
        <div className="bg-white border rounded-xl p-5 shadow-sm flex flex-col items-center justify-center text-center space-y-3">
          <h3 className="text-sm font-bold text-slate-900">Patient QR Code</h3>
          <div className="p-2 border rounded-xl bg-white shadow-sm flex items-center justify-center">
            <canvas ref={canvasRef} />
          </div>
          {qrDataUrl && (
            <a
              href={qrDataUrl}
              download={`${tenant}-booking-qr.png`}
              className="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 text-slate-800 font-medium text-xs rounded-lg transition border"
            >
              📥 Download QR Code
            </a>
          )}
        </div>
      </div>

      {/* Allowed Embed Domains Editor */}
      <div className="bg-white border rounded-xl p-5 shadow-sm space-y-4">
        <div>
          <h3 className="text-sm font-bold text-slate-900">Allowed Website Domains (Frame-Ancestors CSP)</h3>
          <p className="text-xs text-slate-500">
            For security, your booking assistant can only be embedded inside iframes on website domains you explicitly allow.
          </p>
        </div>

        {allowedOrigins.length === 0 && (
          <div className="p-3 bg-amber-50 border border-amber-200 text-amber-900 text-xs rounded-lg font-medium">
            ⚠️ Embedding is off until you add your website&apos;s domain. Your direct link still works.
          </div>
        )}

        <div className="flex flex-col sm:flex-row gap-2">
          <input
            type="text"
            placeholder="e.g. https://myclinic.com or https://*.myclinic.com"
            value={newDomain}
            onChange={(e) => {
              setNewDomain(e.target.value)
              if (domainError) setDomainError('')
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                handleAddDomain()
              }
            }}
            className="flex-1 border rounded-lg p-2 text-xs bg-slate-50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button
            type="button"
            onClick={handleAddDomain}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white font-medium text-xs rounded-lg transition"
          >
            Add Domain
          </button>
        </div>

        {domainError && (
          <div className="text-xs text-red-600 font-medium">{domainError}</div>
        )}

        {allowedOrigins.length > 0 && (
          <div className="space-y-2 pt-2">
            <span className="text-xs font-semibold text-slate-700">Configured Domains ({allowedOrigins.length}/10):</span>
            <div className="flex flex-wrap gap-2">
              {allowedOrigins.map((dom) => (
                <span
                  key={dom}
                  className="inline-flex items-center gap-1.5 px-3 py-1 bg-slate-100 border text-slate-800 text-xs rounded-full font-mono"
                >
                  {dom}
                  <button
                    type="button"
                    onClick={() => handleRemoveDomain(dom)}
                    className="text-slate-400 hover:text-red-600 font-bold ml-1"
                    title="Remove domain"
                  >
                    ×
                  </button>
                </span>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Embed Snippets */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Iframe Snippet */}
        <div className="bg-white border rounded-xl p-5 shadow-sm space-y-3 flex flex-col justify-between">
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-bold text-slate-900">Standard Iframe Embed</h3>
              <button
                onClick={() => handleCopy(iframeSnippet, setCopiedIframe)}
                className="text-xs text-blue-600 hover:underline font-medium"
              >
                {copiedIframe ? '✓ Copied!' : 'Copy Snippet'}
              </button>
            </div>
            <p className="text-xs text-slate-500">
              Embed a responsive inline booking container directly into any page on your website.
            </p>
            <pre className="p-3 bg-slate-900 text-slate-100 text-xs rounded-lg overflow-x-auto whitespace-pre-wrap font-mono">
              {iframeSnippet}
            </pre>
          </div>
        </div>

        {/* Floating Widget Snippet */}
        <div className="bg-white border rounded-xl p-5 shadow-sm space-y-3 flex flex-col justify-between">
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-bold text-slate-900">Floating Popup Widget</h3>
              <button
                onClick={() => handleCopy(widgetSnippet, setCopiedWidget)}
                className="text-xs text-blue-600 hover:underline font-medium"
              >
                {copiedWidget ? '✓ Copied!' : 'Copy Snippet'}
              </button>
            </div>
            <p className="text-xs text-slate-500">
              Paste this lightweight script before the closing &lt;/body&gt; tag to add a floating booking button.
            </p>
            <pre className="p-3 bg-slate-900 text-slate-100 text-xs rounded-lg overflow-x-auto whitespace-pre-wrap font-mono">
              {widgetSnippet}
            </pre>
          </div>
        </div>
      </div>

      {/* Live Preview */}
      <div className="bg-white border rounded-xl p-5 shadow-sm space-y-3">
        <h3 className="text-sm font-bold text-slate-900">Live Embed Preview</h3>
        <p className="text-xs text-slate-500">
          This preview reflects the responsive, chrome-less view loaded inside the embed iframe.
        </p>
        <div className="border rounded-xl overflow-hidden h-[480px] bg-slate-50">
          <iframe
            src={embedLink}
            title={`${clinicName || tenant} Preview`}
            className="w-full h-full border-0"
          />
        </div>
      </div>
    </div>
  )
}
