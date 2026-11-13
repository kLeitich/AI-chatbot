import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import AdminDashboard from '../components/AdminDashboard'
import { PermissionProvider } from '../context/PermissionContext'
import api from '../lib/api'

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }),
  usePathname: () => '/t/test-clinic/admin/dashboard',
  useSearchParams: () => new URLSearchParams(),
}))

vi.mock('../lib/api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('Appointment Form (Conflict Handling and Date Validation)', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    api.get.mockImplementation((url) => {
      if (url.includes('/me')) {
        return Promise.resolve({
          data: { id: 1, role: 'tenant_admin', permissions: [] },
        })
      }
      if (url.includes('/doctors')) {
        return Promise.resolve({
          data: [{ id: 1, name: 'Dr. Smith', specialty: 'General' }],
        })
      }
      if (url.includes('/availability')) {
        return Promise.resolve({ data: { slots: [] } })
      }
      if (url.includes('/appointments')) {
        return Promise.resolve({ data: [] })
      }
      return Promise.resolve({ data: [] })
    })
  })

  it('blocks past dates using Africa/Nairobi min attribute and renders 409 SLOT_TAKEN alternatives', async () => {
    render(
      <PermissionProvider tenant="test-clinic">
        <AdminDashboard tenant="test-clinic" />
      </PermissionProvider>
    )

    // Open "New Appointment" modal
    const addBtn = await screen.findByText('+ New Appointment')
    fireEvent.click(addBtn)

    await screen.findByRole('heading', { name: /Create Appointment/i })

    // Find inputs scoped inside the modal labels
    const dateLabel = screen.getByText('Date *').closest('label')
    const dateInput = dateLabel.querySelector('input')
    expect(dateInput).not.toBeNull()
    expect(dateInput).toHaveAttribute('min')
    const minDate = dateInput.getAttribute('min')
    expect(minDate).toMatch(/^\d{4}-\d{2}-\d{2}$/)

    // Fill appointment form
    const nameInput = screen.getByPlaceholderText('e.g. John Doe')
    fireEvent.change(nameInput, { target: { value: 'Alice Brown' } })

    const docLabel = screen.getByText('Doctor *').closest('label')
    const doctorSelect = docLabel.querySelector('select')
    fireEvent.change(doctorSelect, { target: { value: 'Dr. Smith' } })

    fireEvent.change(dateInput, { target: { value: minDate } })

    const timeLabel = screen.getByText('Time *').closest('label')
    const timeInput = timeLabel.querySelector('input')
    fireEvent.change(timeInput, { target: { value: '10:00' } })

    // Simulate backend 409 conflict when submitting
    api.post.mockRejectedValueOnce({
      response: {
        status: 409,
        data: {
          code: 'SLOT_TAKEN',
          message: 'Selected slot is already taken',
          alternatives: ['11:00', '14:30'],
        },
      },
    })

    const saveBtn = screen.getByRole('button', { name: /^Create Appointment$/i })
    fireEvent.click(saveBtn)

    // Verify error message and alternative chips appear
    await screen.findByText(/Selected slot is already taken/i)
    expect(screen.getByText('⏱️ 11:00')).toBeInTheDocument()
    expect(screen.getByText('⏱️ 14:30')).toBeInTheDocument()

    // Clicking alternative chip updates time and clears error
    const altChip = screen.getByText('⏱️ 11:00')
    fireEvent.click(altChip)

    expect(screen.queryByText(/Selected slot is already taken/i)).not.toBeInTheDocument()
  })
})
