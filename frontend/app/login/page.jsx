import { redirect } from 'next/navigation'

export default function Page() {
  // Backwards-compat: send /login to default tenant login
  redirect('/t/default/login')
}

