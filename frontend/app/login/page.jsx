import { redirect, notFound } from 'next/navigation'

export default function Page() {
  // Legacy route: redirect to /t/default only in development; in production return 404
  if (process.env.NODE_ENV !== 'production') {
    redirect('/t/default/login')
  }
  notFound()
}
