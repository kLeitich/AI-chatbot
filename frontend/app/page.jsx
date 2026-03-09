import { redirect } from 'next/navigation'

export default function Page() {
  // Redirect root to the default tenant chatbot
  redirect('/t/default')
}
