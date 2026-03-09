import { redirect } from 'next/navigation'

export default function Page() {
  // Backwards-compat: send /admin/dashboard to default tenant dashboard
  redirect('/t/default/admin/dashboard')
}
