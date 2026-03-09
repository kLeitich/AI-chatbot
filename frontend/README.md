# Frontend (Next.js + Tailwind)

## Run

```bash
cd frontend
npm install
npm run dev
```

Local URLs:

- Chatbot (default tenant): `http://localhost:3000/t/default`
- Admin login (default tenant): `http://localhost:3000/t/default/login`
- Admin dashboard (default tenant): `http://localhost:3000/t/default/admin/dashboard`

## Environment Variables

Create a `.env` file (optional):
```bash
cp env.example .env
```

- `NEXT_PUBLIC_API_URL` - Backend base URL (default: http://localhost:8080)

## Features

- Interactive chat interface for booking appointments (per tenant)
- Real-time conversation with AI assistant
- Admin panel for managing appointments
- Calendar view for appointments
- Multitenant routing via `/t/{tenant}/...`
- Responsive design with Tailwind CSS

## Development

The frontend uses:
- **Next.js (App Router)** for routing and SSR
- **React** for UI components
- **Tailwind CSS** for styling
- **Axios** for API calls
