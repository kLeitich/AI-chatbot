import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import AdminDashboard from '../components/AdminDashboard'
import AppointmentListView from '../components/AppointmentListView'
import Calendar from '../components/Calendar'
import { PermissionProvider } from '../context/PermissionContext'
import api from '../lib/api'

const mockRouter = { push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }

vi.mock('next/navigation', () => ({
  useRouter: () => mockRouter,
  usePathname: () => '/t/default/admin/dashboard',
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

describe('Dashboard Views and Navigation (Tasks A4, A5, A6, A7)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // -------------------------------------------------------------
  // Part 1: List View tests (Grouping, cards, permissions, empty)
  // -------------------------------------------------------------
  describe('AppointmentListView', () => {
    const today = new Intl.DateTimeFormat('en-CA', {
      timeZone: 'Africa/Nairobi',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).format(new Date())

    const tomorrowObj = new Date(Date.now() + 24 * 60 * 60 * 1000)
    const tomorrow = new Intl.DateTimeFormat('en-CA', {
      timeZone: 'Africa/Nairobi',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).format(tomorrowObj)

    const sampleAppointments = [
      {
        id: 1,
        patient_name: 'Alice Johnson',
        doctor: 'Dr. Smith',
        date: today,
        time: '14:00',
        reason: 'General Checkup',
        status: 'confirmed',
        source: 'instagram',
      },
      {
        id: 2,
        patient_name: 'Bob Williams',
        doctor: 'Dr. Jane',
        date: today,
        time: '09:30',
        reason: 'Follow-up',
        status: 'pending',
      },
      {
        id: 3,
        patient_name: 'Charlie Brown',
        doctor: 'Dr. Smith',
        date: tomorrow,
        time: '11:00',
        reason: 'Consultation',
        status: 'confirmed',
        source: 'widget',
      },
      {
        id: 4,
        patient_name: 'Diana Prince',
        doctor: 'Dr. Strange',
        date: '2027-01-15',
        time: '15:30',
        reason: 'Eye Examination',
        status: 'pending',
      },
    ]

    it('groups appointments by day (Today / Tomorrow / date strings) and sorts by time ascending', () => {
      render(
        <AppointmentListView
          appointments={sampleAppointments}
          rawCount={4}
          canEdit={true}
          canCancel={true}
          canDelete={true}
        />
      )

      expect(screen.getByTestId('appointment-list-view')).toBeInTheDocument()

      // Headers check
      expect(screen.getByText(/Today —/i)).toBeInTheDocument()
      expect(screen.getByText(/Tomorrow —/i)).toBeInTheDocument()
      expect(screen.getByText(/Jan 15, 2027/i)).toBeInTheDocument()

      // Cards rendered
      const cards = screen.getAllByTestId('appointment-card')
      expect(cards).toHaveLength(4)

      // Verify sorting: on Today, Bob (09:30) must appear before Alice (14:00)
      const times = screen.getAllByText(/🕒/)
      expect(times[0].textContent).toContain('09:30')
      expect(times[1].textContent).toContain('14:00')

      // Card details check
      expect(screen.getByText('Alice Johnson')).toBeInTheDocument()
      expect(screen.getByText('instagram')).toBeInTheDocument()
      expect(screen.getByText('widget')).toBeInTheDocument()
    })

    it('gates Edit, Cancel, and Delete actions per permissions', () => {
      const onEdit = vi.fn()
      const onCancel = vi.fn()
      const onDelete = vi.fn()

      // Test with all permissions enabled
      const { rerender } = render(
        <AppointmentListView
          appointments={[sampleAppointments[0]]}
          rawCount={1}
          onEdit={onEdit}
          onCancel={onCancel}
          onDelete={onDelete}
          canEdit={true}
          canCancel={true}
          canDelete={true}
        />
      )

      expect(screen.getByText('Edit')).toBeInTheDocument()
      expect(screen.getByText('Cancel')).toBeInTheDocument()
      expect(screen.getByText('Delete')).toBeInTheDocument()

      fireEvent.click(screen.getByText('Edit'))
      expect(onEdit).toHaveBeenCalledWith(sampleAppointments[0])

      fireEvent.click(screen.getByText('Cancel'))
      expect(onCancel).toHaveBeenCalledWith(sampleAppointments[0])

      fireEvent.click(screen.getByText('Delete'))
      expect(onDelete).toHaveBeenCalledWith(sampleAppointments[0])

      // Rerender with all permissions disabled (e.g. read-only viewer)
      rerender(
        <AppointmentListView
          appointments={[sampleAppointments[0]]}
          rawCount={1}
          onEdit={onEdit}
          onCancel={onCancel}
          onDelete={onDelete}
          canEdit={false}
          canCancel={false}
          canDelete={false}
        />
      )

      expect(screen.queryByText('Edit')).not.toBeInTheDocument()
      expect(screen.queryByText('Cancel')).not.toBeInTheDocument()
      expect(screen.queryByText('Delete')).not.toBeInTheDocument()
    })

    it('renders empty states for no appointments and no filter matches', () => {
      const { rerender } = render(<AppointmentListView appointments={[]} rawCount={0} />)
      expect(screen.getByText('No appointments found')).toBeInTheDocument()
      expect(screen.getByText(/schedule your first clinic appointment/i)).toBeInTheDocument()

      rerender(<AppointmentListView appointments={[]} rawCount={5} />)
      expect(screen.getByText('No appointments match active filters')).toBeInTheDocument()
      expect(screen.getByText(/adjusting or clearing your search filters/i)).toBeInTheDocument()
    })
  })

  // -------------------------------------------------------------
  // Part 2: Calendar Day Mode tests
  // -------------------------------------------------------------
  describe('Calendar Day Mode', () => {
    const fixedDate = new Date('2026-11-20T10:00:00Z')
    const sampleDayAppointments = [
      {
        id: 10,
        patient_name: 'Grace Hopper',
        doctor: 'Dr. Turing',
        date: '2026-11-20',
        time: '10:00',
        reason: 'Algorithm Consultation',
        status: 'confirmed',
      },
      {
        id: 11,
        patient_name: 'Ada Lovelace',
        doctor: 'Dr. Babbage',
        date: '2026-11-20',
        time: '14:00',
        reason: 'Engine Check',
        status: 'pending',
      },
    ]

    it('renders hourly slots per doctor column and handles slot selection', () => {
      const onAdd = vi.fn()
      const onEdit = vi.fn()

      render(
        <Calendar
          date={fixedDate}
          view="day"
          appointments={sampleDayAppointments}
          onAdd={onAdd}
          onEdit={onEdit}
          canCreate={true}
          canEdit={true}
        />
      )

      expect(screen.getByTestId('calendar-day-mode')).toBeInTheDocument()

      // Both doctors appear as column headers
      expect(screen.getByText(/Dr\. Turing/)).toBeInTheDocument()
      expect(screen.getByText(/Dr\. Babbage/)).toBeInTheDocument()

      // Appointment items render in day mode
      expect(screen.getByText(/Grace Hopper/)).toBeInTheDocument()
      expect(screen.getByText(/Ada Lovelace/)).toBeInTheDocument()

      // Test Edit click on Grace Hopper
      const editButtons = screen.getAllByTitle('Edit')
      fireEvent.click(editButtons[0])
      expect(onEdit).toHaveBeenCalledWith(sampleDayAppointments[0])
    })
  })

  // -------------------------------------------------------------
  // Part 3: Error Banner and Retry on /me Failure
  // -------------------------------------------------------------
  describe('AdminDashboard Error Banner & Retry Handling', () => {
    it('displays visible error banner with status code and retry button when /me fails with non-401', async () => {
      // Mock failure first
      api.get.mockImplementation((url) => {
        if (url.includes('/me')) {
          const err = new Error('Database connection failed')
          err.response = {
            status: 503,
            data: {
              code: 'SERVICE_UNAVAILABLE',
              message: 'Database connection failed',
            },
          }
          return Promise.reject(err)
        }
        return Promise.resolve({ data: [] })
      })

      render(
        <PermissionProvider tenant="default">
          <AdminDashboard tenant="default" />
        </PermissionProvider>
      )

      // Alert banner must appear
      const alert = await screen.findByRole('alert')
      expect(alert).toBeInTheDocument()
      expect(screen.getByText('Failed to Load Dashboard Data')).toBeInTheDocument()
      expect(screen.getByText('503')).toBeInTheDocument()
      expect(screen.getByText('SERVICE_UNAVAILABLE')).toBeInTheDocument()
      expect(screen.getByText('Database connection failed')).toBeInTheDocument()

      const retryBtn = screen.getByRole('button', { name: /Retry/i })
      expect(retryBtn).toBeInTheDocument()

      // Now mock successful recovery on retry
      api.get.mockImplementation((url) => {
        if (url.includes('/me')) {
          return Promise.resolve({
            data: {
              id: 1,
              email: 'staff@clinic.com',
              role: 'staff',
              permissions: ['appointments.view'],
            },
          })
        }
        if (url.includes('/appointments')) {
          return Promise.resolve({
            data: [
              {
                id: 101,
                patient_name: 'Recovered Patient',
                doctor: 'Dr. Fixed',
                date: '2026-11-20',
                time: '10:00',
                status: 'confirmed',
              },
            ],
          })
        }
        return Promise.resolve({ data: [] })
      })

      // Click retry
      fireEvent.click(retryBtn)

      // Error banner should be replaced by recovered appointments table
      await waitFor(() => {
        expect(screen.getByText('Recovered Patient')).toBeInTheDocument()
      })
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    })
  })

  // -------------------------------------------------------------
  // Part 4: Tab Visibility and Locked Hints for Presets
  // -------------------------------------------------------------
  describe('Tab Visibility & Access Control for Presets', () => {
    it('viewer: sees Table, List, Calendar, Doctors (read-only); sees locked hints for admin tabs; no New Appointment', async () => {
      api.get.mockImplementation((url) => {
        if (url.includes('/me')) {
          return Promise.resolve({
            data: {
              id: 2,
              email: 'viewer@clinic.com',
              role: 'staff',
              permissions: ['appointments.view', 'doctors.view'],
            },
          })
        }
        if (url.includes('/appointments')) {
          return Promise.resolve({
            data: [
              {
                id: 201,
                patient_name: 'Test Viewer Patient',
                doctor: 'Dr. Jane',
                date: '2026-11-20',
                time: '11:00',
                status: 'confirmed',
              },
            ],
          })
        }
        if (url.includes('/doctors')) {
          return Promise.resolve({
            data: [{ id: 1, name: 'Dr. Jane', specialty: 'General', active: true }],
          })
        }
        return Promise.resolve({ data: [] })
      })

      render(
        <PermissionProvider tenant="default">
          <AdminDashboard tenant="default" />
        </PermissionProvider>
      )

      await screen.findByText('Test Viewer Patient')

      // Viewer cannot create appointments
      expect(screen.queryByText('+ New Appointment')).not.toBeInTheDocument()

      // Accessible tabs
      expect(screen.getByRole('button', { name: 'Table' })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: 'List' })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: 'Calendar' })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: 'Doctors' })).not.toBeDisabled()

      // Clicking Doctors shows read-only doctors directory
      fireEvent.click(screen.getByRole('button', { name: 'Doctors' }))
      expect(await screen.findByTestId('doctor-management')).toBeInTheDocument()
      expect(screen.getByText(/Read-only \(requires doctors\.manage to edit\)/i)).toBeInTheDocument()

      // Locked tabs with hints
      const setupLocked = screen.getByTestId('tab-setup-locked')
      expect(setupLocked).toBeDisabled()
      expect(setupLocked.getAttribute('title')).toContain('reasons.manage')

      const shareLocked = screen.getByTestId('tab-share-locked')
      expect(shareLocked).toBeDisabled()
      expect(shareLocked.getAttribute('title')).toContain('settings.manage')

      const teamLocked = screen.getByTestId('tab-team-locked')
      expect(teamLocked).toBeDisabled()
      expect(teamLocked.getAttribute('title')).toContain('clinic administrator')

      const auditLocked = screen.getByTestId('tab-audit-locked')
      expect(auditLocked).toBeDisabled()
      expect(auditLocked.getAttribute('title')).toContain('clinic administrator')
    })

    it('receptionist: can create appointments and see Doctors (read-only); admin tabs locked', async () => {
      api.get.mockImplementation((url) => {
        if (url.includes('/me')) {
          return Promise.resolve({
            data: {
              id: 3,
              email: 'receptionist@clinic.com',
              role: 'staff',
              permissions: [
                'appointments.view',
                'appointments.create',
                'appointments.edit',
                'appointments.cancel',
                'doctors.view',
              ],
            },
          })
        }
        if (url.includes('/appointments')) {
          return Promise.resolve({ data: [] })
        }
        if (url.includes('/doctors')) {
          return Promise.resolve({ data: [] })
        }
        return Promise.resolve({ data: [] })
      })

      render(
        <PermissionProvider tenant="default">
          <AdminDashboard tenant="default" />
        </PermissionProvider>
      )

      // Receptionist has New Appointment button
      expect(await screen.findByText('+ New Appointment')).toBeInTheDocument()

      // Doctors is accessible
      expect(screen.getByRole('button', { name: 'Doctors' })).not.toBeDisabled()

      // Setup, Share, Team, and Audit are locked
      expect(screen.getByTestId('tab-setup-locked')).toBeDisabled()
      expect(screen.getByTestId('tab-share-locked')).toBeDisabled()
      expect(screen.getByTestId('tab-team-locked')).toBeDisabled()
      expect(screen.getByTestId('tab-audit-locked')).toBeDisabled()
    })

    it('manager: can manage Doctors, Clinic Setup, and Share & Embed; Team & Access and Audit are locked', async () => {
      api.get.mockImplementation((url) => {
        if (url.includes('/me')) {
          return Promise.resolve({
            data: {
              id: 4,
              email: 'manager@clinic.com',
              role: 'staff',
              permissions: [
                'appointments.view',
                'appointments.create',
                'appointments.edit',
                'appointments.cancel',
                'appointments.delete',
                'doctors.view',
                'doctors.manage',
                'reasons.manage',
                'settings.manage',
              ],
            },
          })
        }
        if (url.includes('/appointments')) {
          return Promise.resolve({ data: [] })
        }
        if (url.includes('/doctors')) {
          return Promise.resolve({ data: [] })
        }
        if (url.includes('/reasons')) {
          return Promise.resolve({ data: [] })
        }
        if (url.includes('/settings')) {
          return Promise.resolve({
            data: { emergency_number: '999', booking_link_enabled: true },
          })
        }
        return Promise.resolve({ data: [] })
      })

      render(
        <PermissionProvider tenant="default">
          <AdminDashboard tenant="default" />
        </PermissionProvider>
      )

      expect(await screen.findByText('+ New Appointment')).toBeInTheDocument()

      // Manager has access to Doctors (manage), Clinic Setup, and Share & Embed
      expect(screen.getByRole('button', { name: 'Doctors' })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: 'Clinic Setup' })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: /Share & Embed/i })).not.toBeDisabled()

      // But lacks access to Team & Access and Audit
      expect(screen.getByTestId('tab-team-locked')).toBeDisabled()
      expect(screen.getByTestId('tab-audit-locked')).toBeDisabled()
    })

    it('tenant_admin: has full access to all tabs without any locked tabs', async () => {
      api.get.mockImplementation((url) => {
        if (url.includes('/me')) {
          return Promise.resolve({
            data: {
              id: 5,
              email: 'admin@clinic.com',
              role: 'tenant_admin',
              permissions: [],
            },
          })
        }
        if (url.includes('/appointments')) {
          return Promise.resolve({ data: [] })
        }
        if (url.includes('/doctors')) {
          return Promise.resolve({ data: [] })
        }
        if (url.includes('/reasons')) {
          return Promise.resolve({ data: [] })
        }
        if (url.includes('/settings')) {
          return Promise.resolve({
            data: { emergency_number: '999', booking_link_enabled: true },
          })
        }
        return Promise.resolve({ data: [] })
      })

      render(
        <PermissionProvider tenant="default">
          <AdminDashboard tenant="default" />
        </PermissionProvider>
      )

      expect(await screen.findByText('+ New Appointment')).toBeInTheDocument()

      // All tabs active and clickable
      expect(screen.getByRole('button', { name: 'Table' })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: 'List' })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: 'Calendar' })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: 'Doctors' })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: 'Clinic Setup' })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: /Share & Embed/i })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: /Team & Access/i })).not.toBeDisabled()
      expect(screen.getByRole('button', { name: 'Audit' })).not.toBeDisabled()

      // Zero locked tabs
      expect(screen.queryByTestId('tab-setup-locked')).not.toBeInTheDocument()
      expect(screen.queryByTestId('tab-share-locked')).not.toBeInTheDocument()
      expect(screen.queryByTestId('tab-team-locked')).not.toBeInTheDocument()
      expect(screen.queryByTestId('tab-audit-locked')).not.toBeInTheDocument()
    })
  })
})
