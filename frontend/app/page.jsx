import Link from 'next/link'

export default function Page() {
  return (
    <div className="min-h-screen grid place-items-center p-4 bg-gradient-to-br from-blue-50 to-indigo-100">
      <div className="text-center space-y-6 max-w-xl">
        <h1 className="text-4xl font-bold text-gray-900">AI Doctor Appointment Chatbot</h1>
        <p className="text-lg text-gray-700">Streamline your healthcare appointment booking with AI-powered conversations</p>

        <div className="flex flex-col gap-3 mt-8">
          <Link href="/signup" className="px-6 py-3 rounded-lg bg-blue-600 text-white font-semibold hover:bg-blue-700 text-center">
            Register Your Company
          </Link>
          <Link href="/t/default/login" className="px-6 py-3 rounded-lg border-2 border-blue-600 text-blue-600 font-semibold hover:bg-blue-50 text-center">
            Admin Login
          </Link>
          <Link href="/platform/login" className="px-6 py-3 rounded-lg border-2 border-slate-600 text-slate-700 font-semibold hover:bg-slate-50 text-center">
            Platform Admin Login
          </Link>
          <Link href="/t/default" className="px-6 py-3 rounded-lg text-gray-600 font-semibold hover:underline text-center">
            Try Demo Chatbot
          </Link>
        </div>
      </div>
    </div>
  )
}
