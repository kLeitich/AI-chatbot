import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import UserLogin from '../components/UserLogin'
import api from '../lib/api'

const mockReplace = vi.fn()

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: mockReplace, refresh: vi.fn() }),
  usePathname: () => '/login',
  useSearchParams: () => new URLSearchParams(),
}))

vi.mock('../lib/api', () => ({
  default: {
    post: vi.fn(),
  },
}))

describe('UserLogin Component', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
  })

  it('renders login form with email, password inputs and links', () => {
    render(<UserLogin />)
    expect(screen.getByRole('heading', { name: /sign in/i })).toBeDefined()
    expect(screen.getByPlaceholderText('name@company.com')).toBeDefined()
    expect(screen.getByPlaceholderText('Enter your password')).toBeDefined()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeDefined()
    expect(screen.getByRole('button', { name: /^continue with google$/i })).toBeDefined()
    expect(screen.getByRole('link', { name: /create an account/i })).toBeDefined()
  })

  it('submits credentials to /auth/login and redirects to redirect_url on success', async () => {
    api.post.mockResolvedValueOnce({
      data: {
        token: 'mock-jwt-token',
        role: 'tenant_admin',
        tenant_id: 'skystay-listing-services-kenya-ltd',
        redirect_url: '/t/skystay-listing-services-kenya-ltd/admin/dashboard',
      },
    })

    render(<UserLogin />)

    fireEvent.change(screen.getByPlaceholderText('name@company.com'), {
      target: { value: 'admin2@example.com' },
    })
    fireEvent.change(screen.getByPlaceholderText('Enter your password'), {
      target: { value: 'SkyStayPassword123!' },
    })

    fireEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/auth/login', {
        email: 'admin2@example.com',
        password: 'SkyStayPassword123!',
      })
      expect(localStorage.getItem('token')).toBe('mock-jwt-token')
      expect(localStorage.getItem('tenant_id')).toBe('skystay-listing-services-kenya-ltd')
      expect(localStorage.getItem('role')).toBe('tenant_admin')
    })
  })

  it('handles platform admin login redirect to /platform/dashboard', async () => {
    api.post.mockResolvedValueOnce({
      data: {
        token: 'mock-platform-jwt-token',
        role: 'platform_admin',
        redirect_url: '/platform/dashboard',
      },
    })

    render(<UserLogin />)

    fireEvent.change(screen.getByPlaceholderText('name@company.com'), {
      target: { value: 'superadmin@platform.internal' },
    })
    fireEvent.change(screen.getByPlaceholderText('Enter your password'), {
      target: { value: 'PlatformPass123!' },
    })

    fireEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(localStorage.getItem('token')).toBe('mock-platform-jwt-token')
      expect(localStorage.getItem('role')).toBe('platform_admin')
    })
  })

  it('displays error message on invalid credentials', async () => {
    api.post.mockRejectedValueOnce({
      response: {
        status: 401,
        data: { message: 'Invalid credentials' },
      },
    })

    render(<UserLogin />)

    fireEvent.change(screen.getByPlaceholderText('name@company.com'), {
      target: { value: 'baduser@example.com' },
    })
    fireEvent.change(screen.getByPlaceholderText('Enter your password'), {
      target: { value: 'WrongPass123!' },
    })

    fireEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByText('Invalid credentials')).toBeDefined()
    })
  })
})
