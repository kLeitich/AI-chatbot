import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import Chatbot from '../components/Chatbot'
import api from '../lib/api'

vi.mock('../lib/api', () => ({
  default: {
    post: vi.fn(),
    get: vi.fn(),
  },
}))

describe('Chatbot Component', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    // Mock scrollIntoView which jsdom does not implement
    Element.prototype.scrollIntoView = vi.fn()
  })

  it('renders high-contrast emergency card and tap-to-call buttons when emergency response arrives', async () => {
    api.post.mockResolvedValueOnce({
      data: {
        reply: 'Please seek urgent medical help immediately.',
        emergency: true,
        emergency_category: 'chest_pain',
        emergency_number: '+254711223344',
        session_id: 'sess-123',
      },
    })

    render(<Chatbot tenant="test-clinic" />)

    const input = screen.getByPlaceholderText(/Book me with Dr./i)
    fireEvent.change(input, { target: { value: 'I have severe chest pain and cannot breathe' } })

    const sendBtn = screen.getByRole('button', { name: /Send/i })
    fireEvent.click(sendBtn)

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
    })

    expect(screen.getByText(/Medical Emergency Detected/i)).toBeInTheDocument()
    expect(screen.getByText(/Please seek urgent medical help immediately/i)).toBeInTheDocument()

    // Tap-to-call buttons
    const clinicCallBtn = screen.getByRole('link', { name: /Call Clinic \(\+254711223344\)/i })
    expect(clinicCallBtn).toHaveAttribute('href', 'tel:+254711223344')

    const call999 = screen.getByRole('link', { name: /Call 999/i })
    expect(call999).toHaveAttribute('href', 'tel:999')

    const call112 = screen.getByRole('link', { name: /Call 112/i })
    expect(call112).toHaveAttribute('href', 'tel:112')
  })

  it('clears session id and resets conversation when Start New Conversation is clicked', async () => {
    localStorage.setItem('chat_session_id_test-clinic', 'sess-old-789')

    api.post.mockResolvedValueOnce({
      data: {
        reply: 'Doctor is available at 2pm',
        session_id: 'sess-new-123',
      },
    })

    render(<Chatbot tenant="test-clinic" />)

    const input = screen.getByPlaceholderText(/Book me with Dr./i)
    fireEvent.change(input, { target: { value: 'Check schedule' } })
    fireEvent.click(screen.getByRole('button', { name: /Send/i }))

    await waitFor(() => {
      expect(screen.getByText('Doctor is available at 2pm')).toBeInTheDocument()
    })
    expect(localStorage.getItem('chat_session_id_test-clinic')).toBe('sess-new-123')

    // Click "Start new conversation"
    const newConvBtn = screen.getByRole('button', { name: /Start new conversation/i })
    fireEvent.click(newConvBtn)

    // Verify session in localStorage is removed
    expect(localStorage.getItem('chat_session_id_test-clinic')).toBeNull()

    // Verify conversation was reset to initial greeting
    expect(screen.queryByText('Doctor is available at 2pm')).not.toBeInTheDocument()
    expect(screen.getByText(/Hi! I can book a doctor appointment/i)).toBeInTheDocument()
  })

  it('shows gentle retry message on network / 500 error and preserves history', async () => {
    api.post.mockRejectedValueOnce(new Error('Network error 500'))

    render(<Chatbot tenant="test-clinic" />)

    const input = screen.getByPlaceholderText(/Book me with Dr./i)
    fireEvent.change(input, { target: { value: 'Book with Dr. Jane' } })
    fireEvent.click(screen.getByRole('button', { name: /Send/i }))

    await waitFor(() => {
      expect(
        screen.getByText(/I'm having trouble connecting right now. Please check your connection/i)
      ).toBeInTheDocument()
    })

    // User message is preserved in history
    expect(screen.getByText('Book with Dr. Jane')).toBeInTheDocument()
  })
})

