import { redirect } from 'next/navigation'

export default function Page() {
  // Backwards-compat: send /admin/login to default tenant login
  redirect('/t/default/login')
}
