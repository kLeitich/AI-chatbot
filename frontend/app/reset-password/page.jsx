import { Suspense } from 'react'
import ResetPassword from '../../components/ResetPassword'

export const metadata = {
  title: 'Reset Password - Booka',
}

export default function Page() {
  return (
    <Suspense fallback={
      <div className="min-h-screen grid place-items-center bg-gray-50 text-gray-500 text-sm">
        Loading password reset...
      </div>
    }>
      <ResetPassword />
    </Suspense>
  )
}
