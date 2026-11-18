import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import PlatformDashboardPage from '../app/platform/dashboard/page'
import api from '../lib/api'

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }),
  usePathname: () => '/platform/dashboard',
  useSearchParams: () => new URLSearchParams(),
}))

vi.mock('../lib/api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('Platform Dashboard (Strictly Read-Only)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.get.mockImplementation((url) => {
      if (url.includes('/platform/tenants')) {
        return Promise.resolve({
          data: [
            {
              id: 1,
              name: 'City Health Clinic',
              slug: 'city-health',
              emergency_number: '999',
              emergency_message: 'Emergency line active',
              user_count: 5,
              appointment_count: 42,
              created_at: '2026-01-01',
            },
          ],
        })
      }
      if (url.includes('/platform/users')) {
        return Promise.resolve({
          data: [
            { id: 10, email: 'admin@cityhealth.com', role: 'tenant_admin', status: 'active', tenant_id: 1, created_at: '2026-01-01' },
          ],
        })
      }
      if (url.includes('/platform/doctors')) {
        return Promise.resolve({
          data: [
            { id: 101, name: 'Dr. Sarah', specialty: 'General', tenant_id: 1, active: true },
          ],
        })
      }
      if (url.includes('/platform/appointments')) {
        return Promise.resolve({
          data: { appointments: [], total: 0, page: 1, limit: 20 },
        })
      }
      if (url.includes('/platform/audit')) {
        return Promise.resolve({
          data: { logs: [], total: 0, page: 1, limit: 50 },
        })
      }
      return Promise.resolve({ data: [] })
    })
  })

  it('renders read-only views, executes ONLY GET calls, and provides zero mutation controls', async () => {
    render(<PlatformDashboardPage />)

    await screen.findByText('Platform Overview (Read-Only)')
    expect(screen.getByText('City Health Clinic')).toBeInTheDocument()

    // Switch through all tabs
    const usersTab = screen.getByRole('button', { name: /Users/i })
    fireEvent.click(usersTab)
    await screen.findByText('admin@cityhealth.com')

    const doctorsTab = screen.getByRole('button', { name: /Doctors/i })
    fireEvent.click(doctorsTab)
    await screen.findByText('Dr. Sarah')

    const appointmentsTab = screen.getByRole('button', { name: /Appointments/i })
    fireEvent.click(appointmentsTab)
    await screen.findByText(/No appointments found/i)

    const auditTab = screen.getByRole('button', { name: /Audit Logs/i })
    fireEvent.click(auditTab)
    await screen.findByText(/No audit records found/i)

    // Confirm that NO mutation calls were ever made
    expect(api.post).not.toHaveBeenCalled()
    expect(api.put).not.toHaveBeenCalled()
    expect(api.patch).not.toHaveBeenCalled()
    expect(api.delete).not.toHaveBeenCalled()

    // Confirm that only GET calls were made
    expect(api.get).toHaveBeenCalled()

    // Verify no mutation buttons exist on any view (e.g. Save, Create, Add, Edit, Delete, Update)
    const allButtons = screen.getAllByRole('button')
    for (const btn of allButtons) {
      const text = btn.textContent.toLowerCase()
      expect(text).not.toMatch(/^(add|create|edit|delete|update|save|deactivate|remove)/i)
    }
  })
})

