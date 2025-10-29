import { useEffect, useState } from 'react'

export default function ThemeToggle() {
  const [dark, setDark] = useState(() => {
    const saved = localStorage.getItem('theme')
    if (saved) return saved === 'dark'
    return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
  })

  useEffect(() => {
    const root = document.documentElement
    if (dark) {
      root.classList.add('dark')
      localStorage.setItem('theme', 'dark')
    } else {
      root.classList.remove('dark')
      localStorage.setItem('theme', 'light')
    }
  }, [dark])

  return (
    <button
      aria-label="Toggle dark mode"
      className="fixed bottom-4 right-4 z-50 rounded-full border bg-white text-gray-900 border-gray-200 shadow px-3 py-2 text-sm hover:bg-gray-50 dark:bg-neutral-800 dark:text-neutral-100 dark:border-neutral-700"
      onClick={() => setDark((v) => !v)}
    >
      {dark ? '🌙 Dark' : '☀️ Light'}
    </button>
  )
}
