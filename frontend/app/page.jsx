import Link from 'next/link'

export default function Page() {
  return (
    <div className="min-h-screen bg-gradient-to-b from-slate-50 via-blue-50/30 to-white text-gray-900">
      {/* Navigation Bar */}
      <header className="border-b border-gray-100 bg-white/80 backdrop-blur-md sticky top-0 z-30">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-blue-600 flex items-center justify-center text-white shadow-md shadow-blue-500/20">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
            </div>
            <div>
              <span className="text-xl font-bold tracking-tight text-gray-900">Booka</span>
              <span className="hidden sm:inline-block ml-2 text-xs font-semibold px-2 py-0.5 rounded-full bg-blue-50 text-blue-700 border border-blue-200">
                Healthcare AI
              </span>
            </div>
          </div>

          <nav className="flex items-center gap-3 sm:gap-4">
            <Link
              href="/t/default"
              className="text-sm font-medium text-gray-600 hover:text-blue-600 transition px-3 py-1.5 rounded-lg hover:bg-gray-50"
            >
              Demo Chatbot
            </Link>
            <Link
              href="/login"
              className="text-sm font-semibold text-gray-700 hover:text-blue-600 transition px-4 py-2 rounded-lg border border-gray-200 hover:border-blue-300 hover:bg-blue-50/50"
            >
              Login
            </Link>
            <Link
              href="/signup"
              className="text-sm font-semibold text-white bg-blue-600 hover:bg-blue-700 transition px-4 py-2 rounded-lg shadow-sm shadow-blue-600/30"
            >
              Create an Account
            </Link>
          </nav>
        </div>
      </header>

      {/* Hero Section */}
      <section className="pt-16 pb-20 px-4 sm:px-6 max-w-5xl mx-auto text-center">
        <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-blue-50 border border-blue-200 text-blue-700 text-xs font-medium mb-6">
          <span className="w-2 h-2 rounded-full bg-blue-600 animate-pulse" />
          Conversational Doctor Appointment Scheduling
        </div>

        <h1 className="text-4xl sm:text-5xl md:text-6xl font-extrabold text-gray-900 tracking-tight leading-tight sm:leading-tight">
          Automate Doctor Appointments with{' '}
          <span className="text-transparent bg-clip-text bg-gradient-to-r from-blue-600 to-indigo-600">
            Intelligent AI
          </span>
        </h1>

        <p className="mt-6 text-lg sm:text-xl text-gray-600 max-w-3xl mx-auto leading-relaxed">
          Empower your clinic or hospital with a 24/7 conversational assistant that interacts naturally with patients,
          matches specialists, checks doctor availability, and secures bookings effortlessly.
        </p>

        <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4">
          <Link
            href="/signup"
            className="w-full sm:w-auto px-8 py-3.5 rounded-xl bg-blue-600 text-white font-semibold shadow-lg shadow-blue-600/25 hover:bg-blue-700 transition flex items-center justify-center gap-2 text-base"
          >
            <span>Create an Account</span>
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M14 5l7 7m0 0l-7 7m7-7H3" />
            </svg>
          </Link>

          <Link
            href="/t/default"
            className="w-full sm:w-auto px-8 py-3.5 rounded-xl border-2 border-gray-200 bg-white text-gray-800 font-semibold hover:border-blue-400 hover:text-blue-600 hover:bg-blue-50/30 transition flex items-center justify-center gap-2 text-base"
          >
            <svg className="w-5 h-5 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            <span>Try Demo Chatbot</span>
          </Link>
        </div>

        <p className="mt-4 text-xs text-gray-500">
          Already registered?{' '}
          <Link href="/login" className="text-blue-600 font-medium hover:underline">
            Sign in to your practice dashboard
          </Link>
        </p>
      </section>

      {/* Feature Highlights Grid */}
      <section className="py-16 px-4 sm:px-6 max-w-6xl mx-auto border-t border-gray-100">
        <div className="text-center max-w-2xl mx-auto mb-12">
          <h2 className="text-2xl sm:text-3xl font-bold text-gray-900">
            Everything your healthcare practice needs
          </h2>
          <p className="mt-3 text-gray-600 text-sm sm:text-base">
            Engineered specifically for medical centers, specialized clinics, and private practices.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          <div className="bg-white p-6 rounded-2xl border border-gray-100 shadow-sm hover:shadow-md transition">
            <div className="w-12 h-12 rounded-xl bg-blue-50 text-blue-600 flex items-center justify-center text-xl mb-4 font-bold">
              💬
            </div>
            <h3 className="font-semibold text-gray-900 text-lg mb-2">Conversational AI</h3>
            <p className="text-sm text-gray-600 leading-relaxed">
              Patients chat naturally to describe concerns, select specialists, and book real-time appointments anytime.
            </p>
          </div>

          <div className="bg-white p-6 rounded-2xl border border-gray-100 shadow-sm hover:shadow-md transition">
            <div className="w-12 h-12 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center text-xl mb-4 font-bold">
              🩺
            </div>
            <h3 className="font-semibold text-gray-900 text-lg mb-2">Doctor Schedules</h3>
            <p className="text-sm text-gray-600 leading-relaxed">
              Customize working days, shift hours, consultation durations, and specialties for each doctor independently.
            </p>
          </div>

          <div className="bg-white p-6 rounded-2xl border border-gray-100 shadow-sm hover:shadow-md transition">
            <div className="w-12 h-12 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center text-xl mb-4 font-bold">
              🌐
            </div>
            <h3 className="font-semibold text-gray-900 text-lg mb-2">Embed Anywhere</h3>
            <p className="text-sm text-gray-600 leading-relaxed">
              Drop a single lightweight script into WordPress, Wix, or custom HTML to embed your booking widget directly.
            </p>
          </div>

          <div className="bg-white p-6 rounded-2xl border border-gray-100 shadow-sm hover:shadow-md transition">
            <div className="w-12 h-12 rounded-xl bg-amber-50 text-amber-600 flex items-center justify-center text-xl mb-4 font-bold">
              🔒
            </div>
            <h3 className="font-semibold text-gray-900 text-lg mb-2">Secure Multi-Tenant</h3>
            <p className="text-sm text-gray-600 leading-relaxed">
              Complete data isolation per clinic, role-based access for staff & administrators, and immutable audit logs.
            </p>
          </div>
        </div>
      </section>

      {/* Quick Demo Preview Callout */}
      <section className="py-12 px-4 sm:px-6 max-w-4xl mx-auto">
        <div className="bg-gradient-to-r from-blue-600 to-indigo-700 rounded-3xl p-8 sm:p-12 text-white shadow-xl flex flex-col md:flex-row items-center justify-between gap-8">
          <div className="space-y-3 text-center md:text-left">
            <span className="text-xs font-semibold uppercase tracking-wider text-blue-200">
              Interactive Patient Demo
            </span>
            <h2 className="text-2xl sm:text-3xl font-bold">Experience the booking chatbot live</h2>
            <p className="text-blue-100 text-sm sm:text-base max-w-lg">
              Test how a patient chats with our AI assistant to book with sample doctors (Dr. Kim, Dr. Mercy, Dr. Lee).
            </p>
          </div>

          <Link
            href="/t/default"
            className="px-6 py-3.5 bg-white text-blue-700 font-bold rounded-xl shadow hover:bg-blue-50 transition shrink-0"
          >
            Launch Demo Chatbot
          </Link>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-gray-100 mt-16 py-8 px-4 text-center text-xs text-gray-500">
        <div className="max-w-6xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-4">
          <p>© {new Date().getFullYear()} Booka. Intelligent Healthcare Scheduling. All rights reserved.</p>
          <div className="flex items-center gap-6">
            <Link href="/login" className="hover:text-blue-600 transition">
              Login
            </Link>
            <Link href="/signup" className="hover:text-blue-600 transition">
              Create an Account
            </Link>
            <Link href="/t/default" className="hover:text-blue-600 transition">
              Patient Demo
            </Link>
          </div>
        </div>
      </footer>
    </div>
  )
}
