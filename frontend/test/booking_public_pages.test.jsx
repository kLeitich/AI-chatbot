import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BookTenantPage from '../app/book/[tenant]/page'
import EmbedTenantPage from '../app/embed/[tenant]/page'
import api from '../lib/api'

// Mock next/navigation
let mockParams = { tenant: 'sunrise-clinic' }
let mockSearchParams = new URLSearchParams()

vi.mock('next/navigation', () => ({
  useParams: () => mockParams,
  useSearchParams: () => mockSearchParams,
}))

// Mock api
vi.mock('../lib/api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

describe('Public Booking Pages (/book/:tenant and /embed/:tenant)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockParams = { tenant: 'sunrise-clinic' }
    mockSearchParams = new URLSearchParams()
    localStorage.clear()
    Element.prototype.scrollIntoView = vi.fn()
  })

  it('renders OK state for /book/:tenant with clinic name and Chatbot', async () => {
    api.get.mockResolvedValueOnce({
      data: {
        name: 'Sunrise Clinic',
        slug: 'sunrise-clinic',
        booking_enabled: true,
        emergency_number: '+254700000000',
        privacy_url: 'https://sunrise.com/privacy',
      },
    })

    render(<BookTenantPage />)

    // Should load and display clinic name
    const headings = await screen.findAllByRole('heading', { name: 'Sunrise Clinic' })
    expect(headings.length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText(/Book your appointment with our medical team/i)).toBeInTheDocument()

    // AI disclosure should be rendered with privacy link
    expect(screen.getByText(/This assistant collects your name/i)).toBeInTheDocument()
    const privacyLink = await screen.findByRole('link', { name: /Privacy Policy/i })
    expect(privacyLink).toHaveAttribute('href', 'https://sunrise.com/privacy')
  })

  it('renders "Clinic not found" when public-config returns 404', async () => {
    api.get.mockRejectedValueOnce({
      response: { status: 404, data: { code: 'NOT_FOUND' } },
    })

    render(<BookTenantPage />)

    expect(await screen.findByText('Clinic not found')).toBeInTheDocument()
    expect(screen.getByText(/does not exist or may have been moved/i)).toBeInTheDocument()
  })

  it('renders "Online booking is unavailable" and emergency call button when booking_enabled is false', async () => {
    api.get.mockResolvedValueOnce({
      data: {
        name: 'Sunrise Clinic',
        slug: 'sunrise-clinic',
        booking_enabled: false,
        emergency_number: '+254711999888',
      },
    })

    render(<BookTenantPage />)

    expect(await screen.findByText('Online booking is unavailable')).toBeInTheDocument()
    expect(screen.getByText(/Online booking is currently disabled for Sunrise Clinic/i)).toBeInTheDocument()
    const callLink = screen.getByRole('link', { name: /Call \+254711999888/i })
    expect(callLink).toHaveAttribute('href', 'tel:+254711999888')
  })

  it('passes ?doctor=, ?reason=, and ?source= as context on the first chat message only', async () => {
    api.get.mockResolvedValueOnce({
      data: {
        name: 'Sunrise Clinic',
        slug: 'sunrise-clinic',
        booking_enabled: true,
        emergency_number: '999',
      },
    })

    // Set query parameters
    mockSearchParams = new URLSearchParams('doctor=4&reason=2&source=instagram')

    api.post.mockResolvedValueOnce({
      data: {
        reply: 'Hello! I see you want to see Dr. 4 for reason 2.',
        session_id: 'sess-first',
      },
    })

    render(<BookTenantPage />)

    const headings = await screen.findAllByRole('heading', { name: 'Sunrise Clinic' })
    expect(headings.length).toBeGreaterThanOrEqual(1)

    const input = screen.getByPlaceholderText(/Book me with Dr./i)
    fireEvent.change(input, { target: { value: 'Hi there' } })
    const sendBtn = screen.getByRole('button', { name: /Send/i })
    fireEvent.click(sendBtn)

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith(
        '/t/sunrise-clinic/chat',
        {
          message: 'Hi there',
          context: {
            doctor_id: 4,
            reason_id: 2,
            source: 'instagram',
          },
        }
      )
    })

    // Second message should NOT send context again
    api.post.mockResolvedValueOnce({
      data: {
        reply: 'Sure, what time works?',
        session_id: 'sess-first',
      },
    })

    fireEvent.change(input, { target: { value: 'Tomorrow at 10am' } })
    fireEvent.click(sendBtn)

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith(
        '/t/sunrise-clinic/chat',
        {
          message: 'Tomorrow at 10am',
          session_id: 'sess-first',
        }
      )
    })
  })

  it('renders embed page states (/embed/:tenant) correctly', async () => {
    // 1. Embed OK state
    api.get.mockResolvedValueOnce({
      data: {
        name: 'Embed Clinic',
        slug: 'sunrise-clinic',
        booking_enabled: true,
      },
    })

    const { unmount } = render(<EmbedTenantPage />)
    expect(await screen.findByRole('heading', { name: 'Embed Clinic' })).toBeInTheDocument()
    unmount()

    // 2. Embed disabled state
    api.get.mockResolvedValueOnce({
      data: {
        name: 'Embed Clinic',
        slug: 'sunrise-clinic',
        booking_enabled: false,
        emergency_number: '112',
      },
    })

    render(<EmbedTenantPage />)
    expect(await screen.findByText('Online booking is unavailable')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Call 112/i })).toHaveAttribute('href', 'tel:112')
  })
})
