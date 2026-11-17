import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ShareAndEmbed, { validateOriginClient, escapeHtml } from '../components/ShareAndEmbed'
import api from '../lib/api'

vi.mock('../lib/api', () => ({
  default: {
    get: vi.fn(),
    patch: vi.fn(),
  },
}))

describe('ShareAndEmbed Component and Utilities', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.get.mockImplementation((url) => {
      if (url.includes('/admin/settings')) {
        return Promise.resolve({
          data: {
            booking_link_enabled: true,
            allowed_embed_origins: ['https://example.com'],
            privacy_url: 'https://example.com/privacy',
          },
        })
      }
      if (url.includes('/public-config')) {
        return Promise.resolve({
          data: {
            name: 'Demo Health & Care',
            slug: 'demo-clinic',
          },
        })
      }
      return Promise.resolve({ data: {} })
    })
  })

  describe('HTML escaping and snippet generation', () => {
    it('properly escapes special characters', () => {
      expect(escapeHtml('<script>alert("xss")&\'</script>')).toBe(
        '&lt;script&gt;alert(&quot;xss&quot;)&amp;&#39;&lt;/script&gt;'
      )
    })

    it('renders correct direct link, iframe snippet, and widget snippet with escaping', async () => {
      render(<ShareAndEmbed tenant="demo-clinic" />)

      await screen.findByText('Share & Embed Clinic Booking')

      // Direct link input
      const directInput = screen.getByDisplayValue(/http.*\/book\/demo-clinic/)
      expect(directInput).toBeInTheDocument()

      // Iframe snippet
      const iframeSnippet = screen.getByText(/<iframe src=/i)
      expect(iframeSnippet.textContent).toContain('title="Demo Health &amp; Care - Online Appointment Booking"')
      expect(iframeSnippet.textContent).toContain('width="100%"')
      expect(iframeSnippet.textContent).toContain('height="640"')
      expect(iframeSnippet.textContent).toContain('loading="lazy"')

      // Floating widget snippet
      const widgetSnippet = screen.getByText(/<script src=/i)
      expect(widgetSnippet.textContent).toContain('data-clinic="demo-clinic"')
      expect(widgetSnippet.textContent).toContain('/embed.js')
    })

    it('updates snippets dynamically when query parameters are entered', async () => {
      render(<ShareAndEmbed tenant="demo-clinic" />)

      await screen.findByText('Share & Embed Clinic Booking')

      const sourceInput = screen.getByPlaceholderText(/instagram, newsletter/i)
      fireEvent.change(sourceInput, { target: { value: 'facebook' } })

      const doctorInput = screen.getByPlaceholderText(/^e\.g\. 1$/i)
      fireEvent.change(doctorInput, { target: { value: '7' } })

      // Snippets should contain query parameters
      await waitFor(() => {
        const iframeSnippet = screen.getByText(/<iframe src=/i)
        expect(iframeSnippet.textContent).toContain('source=facebook')
        expect(iframeSnippet.textContent).toContain('doctor=7')
      })
    })

    it('shows warning when allowed origins list is empty', async () => {
      api.get.mockImplementation((url) => {
        if (url.includes('/admin/settings')) {
          return Promise.resolve({
            data: {
              booking_link_enabled: true,
              allowed_embed_origins: [],
              privacy_url: '',
            },
          })
        }
        return Promise.resolve({ data: { name: 'Empty Clinic' } })
      })

      render(<ShareAndEmbed tenant="demo-clinic" />)

      expect(
        await screen.findByText(/Embedding is off until you add your website's domain\. Your direct link still works\./i)
      ).toBeInTheDocument()
    })
  })

  describe('Origin Validation Table', () => {
    it('validates empty origin', () => {
      expect(validateOriginClient('')).toEqual({ valid: false, error: 'origin cannot be empty' })
      expect(validateOriginClient('   ')).toEqual({ valid: false, error: 'origin cannot be empty' })
    })

    it('rejects trailing slashes, paths, and queries', () => {
      expect(validateOriginClient('https://example.com/')).toEqual({
        valid: false,
        error: 'origin must not contain paths, queries, fragments or trailing slashes',
      })
      expect(validateOriginClient('https://example.com?foo=bar')).toEqual({
        valid: false,
        error: 'origin must not contain paths, queries, fragments or trailing slashes',
      })
      expect(validateOriginClient('https://example.com#section')).toEqual({
        valid: false,
        error: 'origin must not contain paths, queries, fragments or trailing slashes',
      })
    })

    it('rejects non-https schemes (except localhost in dev)', () => {
      expect(validateOriginClient('http://example.com', false)).toEqual({
        valid: false,
        error: 'origin must use https://',
      })
      expect(validateOriginClient('http://localhost:3000', false)).toEqual({
        valid: false,
        error: 'http://localhost is only allowed in development environment',
      })
      // Dev mode allows http://localhost
      expect(validateOriginClient('http://localhost:3000', true)).toEqual({
        valid: true,
        origin: 'http://localhost:3000',
      })
      expect(validateOriginClient('http://localhost', true)).toEqual({
        valid: true,
        origin: 'http://localhost',
      })
      expect(validateOriginClient('http://localhost:abc', true)).toEqual({
        valid: false,
        error: 'invalid port number on localhost',
      })
    })

    it('validates wildcard domains strictly', () => {
      expect(validateOriginClient('https://*.example.com')).toEqual({
        valid: true,
        origin: 'https://*.example.com',
      })
      expect(validateOriginClient('https://sub.*.example.com')).toEqual({
        valid: false,
        error: "wildcard '*' is only allowed as the leading label, e.g. https://*.example.com",
      })
      expect(validateOriginClient('https://*.*.example.com')).toEqual({
        valid: false,
        error: 'multiple wildcards are not allowed',
      })
    })

    it('validates ports properly', () => {
      expect(validateOriginClient('https://example.com:8443')).toEqual({
        valid: true,
        origin: 'https://example.com:8443',
      })
      expect(validateOriginClient('https://example.com:0')).toEqual({
        valid: false,
        error: 'invalid port number',
      })
      expect(validateOriginClient('https://example.com:70000')).toEqual({
        valid: false,
        error: 'invalid port number',
      })
    })
  })
})
