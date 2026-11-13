import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ClinicSetup from '../components/ClinicSetup'
import { PermissionProvider } from '../context/PermissionContext'
import api from '../lib/api'

vi.mock('../lib/api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('ClinicSetup Component', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    window.confirm = vi.fn(() => true)
  })

  const adminProfile = {
    id: 1,
    role: 'tenant_admin',
    permissions: [],
  }

  const sampleDoctors = [
    {
      id: 1,
      name: 'Dr. John Doe',
      specialty: 'Cardiology',
      work_days: 'Mon,Tue,Wed',
      work_start: '09:00',
      work_end: '17:00',
      slot_minutes: 30,
      active: true,
    },
  ]

  const sampleReasons = [
    { id: 10, label: 'General Checkup', active: true },
  ]

  const sampleSettings = {
    emergency_number: '+254711223344',
    emergency_message: 'Please contact emergency services immediately.',
  }

  function renderWithAdmin(ui) {
    api.get.mockImplementation((url) => {
      if (url.includes('/me')) return Promise.resolve({ data: adminProfile })
      if (url.includes('/admin/doctors')) return Promise.resolve({ data: sampleDoctors })
      if (url.includes('/admin/reasons')) return Promise.resolve({ data: sampleReasons })
      if (url.includes('/admin/settings')) return Promise.resolve({ data: sampleSettings })
      return Promise.resolve({ data: {} })
    })

    return render(<PermissionProvider tenant="test-clinic">{ui}</PermissionProvider>)
  }

  it('renders doctors, handles add, edit, and deactivate doctor', async () => {
    api.post.mockResolvedValueOnce({ data: { id: 2, name: 'Dr. Jane' } })
    api.patch.mockResolvedValueOnce({ data: { success: true } })

    renderWithAdmin(<ClinicSetup tenant="test-clinic" />)

    await screen.findByText('Dr. John Doe')
    expect(screen.getByText('Cardiology')).toBeInTheDocument()

    // Add Doctor
    const nameInput = screen.getByPlaceholderText(/Dr\. Jane Smith/i)
    fireEvent.change(nameInput, { target: { value: 'Dr. Jane' } })
    const addBtn = screen.getByRole('button', { name: /^Add Doctor$/i })
    fireEvent.click(addBtn)

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith(
        '/t/test-clinic/admin/doctors',
        expect.objectContaining({ name: 'Dr. Jane' })
      )
    })

    // Deactivate Doctor (first Deactivate button on page)
    const deactivateBtns = screen.getAllByRole('button', { name: /Deactivate/i })
    fireEvent.click(deactivateBtns[0])

    await waitFor(() => {
      expect(api.patch).toHaveBeenCalledWith(
        '/t/test-clinic/admin/doctors/1',
        { active: false }
      )
    })
  })

  it('handles add and deactivate visit reasons', async () => {
    api.post.mockResolvedValueOnce({ data: { id: 11, label: 'Dental Cleaning' } })
    api.patch.mockResolvedValueOnce({ data: { success: true } })

    renderWithAdmin(<ClinicSetup tenant="test-clinic" />)

    await screen.findByText('General Checkup')

    // Add Reason
    const reasonInput = screen.getByPlaceholderText(/e\.g\. Dental Checkup/i)
    fireEvent.change(reasonInput, { target: { value: 'Dental Cleaning' } })
    const addReasonBtn = screen.getByRole('button', { name: /^Add Reason$/i })
    fireEvent.click(addReasonBtn)

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith(
        '/t/test-clinic/admin/reasons',
        { label: 'Dental Cleaning' }
      )
    })

    // Deactivate Reason (second Deactivate button on page)
    const deactivateBtns = screen.getAllByRole('button', { name: /Deactivate/i })
    fireEvent.click(deactivateBtns[1])

    await waitFor(() => {
      expect(api.patch).toHaveBeenCalledWith(
        '/t/test-clinic/admin/reasons/10',
        { active: false }
      )
    })
  })

  it('handles emergency settings update and displays validation error on invalid input', async () => {
    api.patch.mockRejectedValueOnce({
      response: {
        data: {
          code: 'VALIDATION_ERROR',
          message: 'emergency_number contains invalid characters',
        },
      },
    })

    renderWithAdmin(<ClinicSetup tenant="test-clinic" />)

    await screen.findByDisplayValue('+254711223344')

    const saveBtn = screen.getByRole('button', { name: /Save Emergency Settings/i })
    fireEvent.click(saveBtn)

    await waitFor(() => {
      expect(screen.getByText(/emergency_number contains invalid characters/i)).toBeInTheDocument()
    })
  })
})
