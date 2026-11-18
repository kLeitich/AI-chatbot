import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PermissionProvider, usePermissions } from '../context/PermissionContext'
import AdminDashboard from '../components/AdminDashboard'
import AppointmentTable from '../components/AppointmentTable'
import Calendar from '../components/Calendar'
import api from '../lib/api'

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }),
  usePathname: () => '/t/default/admin/dashboard',
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

const sampleAppointments = [
  {
    id: 1,
    patient_name: 'Jane Doe',
    doctor: 'Dr. Smith',
    date: '2026-11-15',
    time: '10:00',
    reason: 'Routine Checkup',
    status: 'confirmed',
  },
]

function TestConsumer({ checkPerm }) {
  const { can, role } = usePermissions()
  return (
    <div>
      <span data-testid="role">{role}</span>
      <span data-testid="allowed">{can(checkPerm) ? 'yes' : 'no'}</span>
    </div>
  )
}

describe('Permission Matrix and View Guards', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('evaluates viewer permissions correctly', async () => {
    api.get.mockResolvedValueOnce({
      data: {
        id: 1,
        role: 'staff',
        permissions: ['appointments.view', 'doctors.view'],
      },
    })
    render(
      <PermissionProvider tenant="default">
        <TestConsumer checkPerm="appointments.create" />
      </PermissionProvider>
    )
    await screen.findByText('no')
  })

  it('evaluates receptionist permissions correctly', async () => {
    api.get.mockResolvedValueOnce({
      data: {
        id: 2,
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
    render(
      <PermissionProvider tenant="default">
        <TestConsumer checkPerm="appointments.create" />
      </PermissionProvider>
    )
    await screen.findByText('yes')
  })

  it('evaluates manager preset permissions correctly', async () => {
    function MultiCheck() {
      const { can } = usePermissions()
      return (
        <div>
          <span data-testid="can-delete">{can('appointments.delete') ? 'yes' : 'no'}</span>
          <span data-testid="can-doc-manage">{can('doctors.manage') ? 'yes' : 'no'}</span>
          <span data-testid="can-settings">{can('settings.manage') ? 'yes' : 'no'}</span>
        </div>
      )
    }

    api.get.mockResolvedValueOnce({
      data: {
        id: 3,
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
        ],
      },
    })

    render(
      <PermissionProvider tenant="default">
        <MultiCheck />
      </PermissionProvider>
    )

    await screen.findByTestId('can-delete')
    expect(screen.getByTestId('can-delete').textContent).toBe('yes')
    expect(screen.getByTestId('can-doc-manage').textContent).toBe('yes')
    expect(screen.getByTestId('can-settings').textContent).toBe('no')
  })

  it('evaluates tenant_admin permissions correctly for any permission', async () => {
    api.get.mockResolvedValueOnce({
      data: {
        id: 4,
        role: 'tenant_admin',
        permissions: [],
      },
    })
    render(
      <PermissionProvider tenant="default">
        <TestConsumer checkPerm="custom.unlisted.permission" />
      </PermissionProvider>
    )
    await screen.findByText('yes')
  })

  it('renders AppointmentTable action controls strictly according to permissions', () => {
    const onEdit = vi.fn()
    const onCancel = vi.fn()
    const onDelete = vi.fn()

    // Viewer: canView=true, canEdit=false, canCancel=false, canDelete=false
    const { rerender } = render(
      <AppointmentTable
        appointments={sampleAppointments}
        canEdit={false}
        canCancel={false}
        canDelete={false}
        onEdit={onEdit}
        onCancel={onCancel}
        onDelete={onDelete}
      />
    )

    expect(screen.getByText('Jane Doe')).toBeInTheDocument()
    expect(screen.queryByText('Edit')).not.toBeInTheDocument()
    expect(screen.queryByText('Cancel')).not.toBeInTheDocument()
    expect(screen.queryByText('Delete')).not.toBeInTheDocument()

    // Receptionist: canEdit=true, canCancel=true, canDelete=false
    rerender(
      <AppointmentTable
        appointments={sampleAppointments}
        canEdit={true}
        canCancel={true}
        canDelete={false}
        onEdit={onEdit}
        onCancel={onCancel}
        onDelete={onDelete}
      />
    )
    expect(screen.getByText('Edit')).toBeInTheDocument()
    expect(screen.getByText('Cancel')).toBeInTheDocument()
    expect(screen.queryByText('Delete')).not.toBeInTheDocument()

    // Manager / Tenant Admin: canEdit=true, canCancel=true, canDelete=true
    rerender(
      <AppointmentTable
        appointments={sampleAppointments}
        canEdit={true}
        canCancel={true}
        canDelete={true}
        onEdit={onEdit}
        onCancel={onCancel}
        onDelete={onDelete}
      />
    )
    expect(screen.getByText('Edit')).toBeInTheDocument()
    expect(screen.getByText('Cancel')).toBeInTheDocument()
    expect(screen.getByText('Delete')).toBeInTheDocument()
  })

  it('renders Calendar action controls according to viewer, receptionist, manager, and tenant_admin permissions', () => {
    const onEdit = vi.fn()
    const onCancel = vi.fn()
    const onDelete = vi.fn()
    const onAdd = vi.fn()

    const targetDate = new Date('2026-11-15T10:00:00')

    // 1. Viewer: canCreate=false, canEdit=false, canCancel=false, canDelete=false
    const { rerender } = render(
      <Calendar
        date={targetDate}
        view="month"
        appointments={sampleAppointments}
        onAdd={onAdd}
        onEdit={onEdit}
        onCancel={onCancel}
        onDelete={onDelete}
        canCreate={false}
        canEdit={false}
        canCancel={false}
        canDelete={false}
      />
    )
    expect(screen.getByText(/Jane Doe/i)).toBeInTheDocument()
    expect(screen.queryByTitle('Edit')).not.toBeInTheDocument()
    expect(screen.queryByTitle('Cancel')).not.toBeInTheDocument()
    expect(screen.queryByTitle('Delete')).not.toBeInTheDocument()

    // 2. Receptionist: canCreate=true, canEdit=true, canCancel=true, canDelete=false
    rerender(
      <Calendar
        date={targetDate}
        view="month"
        appointments={sampleAppointments}
        onAdd={onAdd}
        onEdit={onEdit}
        onCancel={onCancel}
        onDelete={onDelete}
        canCreate={true}
        canEdit={true}
        canCancel={true}
        canDelete={false}
      />
    )
    expect(screen.getByTitle('Edit')).toBeInTheDocument()
    expect(screen.getByTitle('Cancel')).toBeInTheDocument()
    expect(screen.queryByTitle('Delete')).not.toBeInTheDocument()

    // 3. Manager: canCreate=true, canEdit=true, canCancel=true, canDelete=true
    rerender(
      <Calendar
        date={targetDate}
        view="month"
        appointments={sampleAppointments}
        onAdd={onAdd}
        onEdit={onEdit}
        onCancel={onCancel}
        onDelete={onDelete}
        canCreate={true}
        canEdit={true}
        canCancel={true}
        canDelete={true}
      />
    )
    expect(screen.getByTitle('Edit')).toBeInTheDocument()
    expect(screen.getByTitle('Cancel')).toBeInTheDocument()
    expect(screen.getByTitle('Delete')).toBeInTheDocument()
  })

  it('hides New Appointment, Team tab, and Audit tab on AdminDashboard for viewer', async () => {
    api.get.mockImplementation((url) => {
      if (url.includes('/me')) {
        return Promise.resolve({
          data: {
            id: 1,
            role: 'staff',
            permissions: ['appointments.view'],
          },
        })
      }
      if (url.includes('/appointments')) {
        return Promise.resolve({ data: sampleAppointments })
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

    await screen.findByText('Jane Doe')
    // Viewer should NOT see "New Appointment" button
    expect(screen.queryByText('+ New Appointment')).not.toBeInTheDocument()
    // Viewer should NOT see Team, Audit, or Clinic Setup tabs
    expect(screen.queryByText('Team')).not.toBeInTheDocument()
    expect(screen.queryByText('Audit')).not.toBeInTheDocument()
    expect(screen.queryByText('Clinic Setup')).not.toBeInTheDocument()
  })

  it('shows New Appointment, Clinic Setup, Team, and Audit tabs for tenant_admin', async () => {
    api.get.mockImplementation((url) => {
      if (url.includes('/me')) {
        return Promise.resolve({
          data: {
            id: 10,
            role: 'tenant_admin',
            permissions: [],
          },
        })
      }
      if (url.includes('/appointments')) {
        return Promise.resolve({ data: sampleAppointments })
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

    await screen.findByText('+ New Appointment')
    expect(screen.getByText('Team')).toBeInTheDocument()
    expect(screen.getByText('Audit')).toBeInTheDocument()
    expect(screen.getByText('Clinic Setup')).toBeInTheDocument()
  })
})
