import { describe, it, expect, beforeEach } from 'vitest'
import fs from 'fs'
import path from 'path'

describe('Standalone embed.js Widget', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    delete window.__booka_embed
  })

  it('reads data-clinic, builds expected iframe URL, injects button and modal, and handles toggle/Esc', () => {
    // Setup script element in DOM
    const scriptEl = document.createElement('script')
    scriptEl.src = 'https://portal.myclinic.org/embed.js'
    scriptEl.setAttribute('data-clinic', 'springfield-health')
    scriptEl.setAttribute('data-position', 'left')
    scriptEl.setAttribute('data-color', '#10b981')
    document.body.appendChild(scriptEl)

    // Read script source
    const scriptPath = path.resolve(__dirname, '../public/embed.js')
    const scriptCode = fs.readFileSync(scriptPath, 'utf-8')

    // Execute script
    eval(scriptCode)

    // Button should be injected
    const btn = document.getElementById('booka-widget-button')
    expect(btn).toBeInTheDocument()
    expect(btn.getAttribute('aria-label')).toBe('Open appointment booking chat')
    expect(btn.getAttribute('aria-expanded')).toBe('false')
    expect(btn.style.left).toBe('20px')
    expect(btn.style.backgroundColor).toBe('rgb(16, 185, 129)')

    // Container dialog should be injected and hidden
    const container = document.getElementById('booka-widget-container')
    expect(container).toBeInTheDocument()
    expect(container.style.display).toBe('none')
    expect(container.style.left).toBe('20px')

    // Iframe should be created with expected URL
    const iframe = container.querySelector('iframe')
    expect(iframe).toBeInTheDocument()
    expect(iframe.src).toBe('https://portal.myclinic.org/embed/springfield-health?source=widget')
    expect(iframe.title).toBe('Clinic Appointment Booking')

    // Click button to open
    btn.click()
    expect(container.style.display).toBe('block')
    expect(btn.getAttribute('aria-expanded')).toBe('true')
    expect(btn.getAttribute('aria-label')).toBe('Close appointment booking chat')

    // Press Escape to close
    const escEvent = new KeyboardEvent('keydown', { key: 'Escape' })
    document.dispatchEvent(escEvent)
    expect(container.style.display).toBe('none')
    expect(btn.getAttribute('aria-expanded')).toBe('false')
  })
})
