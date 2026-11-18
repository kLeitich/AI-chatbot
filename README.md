
# AI-Powered Doctor Appointment Chatbot

A Go + Next.js + Go/Fiber full-stack chatbot integrated with the Groq API to handle natural language appointment scheduling with intelligent conversation state management.

## Features
- Natural chat-driven booking with conversational memory
- AI-driven extraction of doctor, date, time, patient name, and reason
- Smart conversation state tracking (never asks for information already provided)
- Automatic time format conversion (4pm → 16:00)
- Context-aware responses that remember previous messages
- Admin panel for appointment management (table + calendar views)
- Calendar dashboard to view and manage appointments (create/edit/delete)
- **Multitenant architecture** (multiple clinics via `/t/{tenant}/...` routes)
- Cloud LLM integration (Groq API)
- Dockerized full stack
- Automatic .env file loading

## System Requirements

| Component | Version |
|---|---|
| Go | 1.20+ |
| Node.js | 18+ |
| Groq API Key | Required ([Get one here](https://console.groq.com/)) |
| Docker | 24+ (optional) |

## Environment Variables

The backend automatically loads environment variables from a `.env` file if it exists. Create per-service env files by copying the examples:

```bash
# Backend
cp backend/env.example backend/.env
# Then edit backend/.env and add your GROQ_API_KEY

# Frontend
cp frontend/env.example frontend/.env
```

**Backend (.env)**:
- `APP_ENV` (Set to `development` locally, `production` on Render; if unset, fails closed to `production`. Valid values: `development`, `staging`, `production`)
- `PORT` (default: 8080)
- `JWT_SECRET` (**required in production**: minimum 32 characters, no default fallbacks)
- `TRUST_PROXY` (set to `true` on Render or behind reverse proxies to enable proper client IP rate limiting via X-Forwarded-For)
- `GROQ_API_KEY` (**required** - get from [Groq Console](https://console.groq.com/))
- `GROQ_MODEL` (default: llama-3.3-70b-versatile)
- `SQLITE_PATH` (default: appointments.db, use `/data/appointments.db` on Render)
- `DEFAULT_ADMIN_EMAIL` (optional seed email in development)
- `DEFAULT_ADMIN_PASSWORD` (optional seed password in development)
- `FRONTEND_URL` (default: http://localhost:3000 for local dev) - **Required in production**: Set to your Vercel URL (e.g., https://ai-chatbot-gamma-blue-98.vercel.app)

**Frontend (.env)** (Next.js): 
- `NEXT_PUBLIC_API_URL` (default: http://localhost:8080 for local dev) - **Required in production**: Set to your Render backend URL (e.g., https://ai-chatbot-1vkx.onrender.com)

**Note**: Both services work out of the box for local development. No environment variables are required for local testing. For production deployment, set the environment variables as described in the Production Deployment section below.

## Installation

### Backend

1. **Get a Groq API Key**:
   - Sign up at [Groq Console](https://console.groq.com/)
   - Create an API key

2. **Setup environment**:
   ```bash
   cd backend
   cp env.example .env
   # Edit .env and add your GROQ_API_KEY
   ```

3. **Run the backend**:
   ```bash
   go mod tidy
   go run .
   ```

### Frontend (Next.js App Router)
```bash
cd frontend
npm install
npm run dev
```

Local URLs:

- Chatbot (default tenant): `http://localhost:3000/t/default`
- Admin login (default tenant): `http://localhost:3000/t/default/login`
- Admin dashboard (default tenant): `http://localhost:3000/t/default/admin/dashboard`

## Share your booking link

Clinics can share their appointment booking assistant directly with patients or embed it into their existing website using three integration options:

### 1. Direct Booking Page
Send your patients directly to your dedicated full-page booking link:
```text
https://your-domain.com/book/<clinic-slug>
```
You can also append URL parameters to pre-fill context or track marketing attribution:
- Pre-select a doctor: `https://your-domain.com/book/<clinic-slug>?doctor=2`
- Pre-select a reason: `https://your-domain.com/book/<clinic-slug>?reason=1`
- Track campaign source: `https://your-domain.com/book/<clinic-slug>?source=instagram`

### 2. Website Iframe Embed
Embed a responsive, chrome-less booking container inside any webpage on your clinic's site:
```html
<iframe
  src="https://your-domain.com/embed/<clinic-slug>"
  title="Clinic Appointment Booking"
  width="100%"
  height="640"
  loading="lazy"
  style="border:none; border-radius:12px;">
</iframe>
```

### 3. Floating Popup Widget
Add an accessible floating chat widget button to your site by including `embed.js` before your closing `</body>` tag:
```html
<script
  src="https://your-domain.com/embed.js"
  data-clinic="<clinic-slug>"
  data-position="right"
  data-color="#2563eb">
</script>
```

### Domain Allowlist Requirement (Security Note)
> **Important:** To protect patient privacy and prevent clickjacking, embedded routes (`/embed/<clinic-slug>`) enforce strict Content Security Policy (`frame-ancestors`).
> Before embedding the booking page or floating widget on your website, a clinic administrator must add your website domain (e.g., `https://myclinic.com` or `https://*.myclinic.com`) to the **Allowed Embed Domains** list in the Admin Dashboard (**Clinic Setup** / **Share & Embed**).
> If no domains are configured, iframe embedding is blocked (`frame-ancestors 'none'`) by default, while your direct booking link (`/book/<clinic-slug>`) continues to function normally.


## Docker Deployment
```bash
docker-compose up --build
```

Access:
- Frontend → http://localhost
- Backend → http://localhost:8080

**Note**: For Docker deployment, ensure `GROQ_API_KEY` is set in the backend service environment variables.

## Production Deployment

### Deploying to Render (Backend)

1. Connect your GitHub repository to Render
2. Set the following environment variables in Render dashboard:
   - `APP_ENV=production`
   - `PORT=8080` (Render will override this automatically)
   - `TRUST_PROXY=true` (Required on Render so Fiber reads client IPs from X-Forwarded-For for accurate rate limiting)
   - `GROQ_API_KEY` - Your Groq API key
   - `FRONTEND_URL` - Your Vercel frontend URL (e.g., `https://ai-chatbot-gamma-blue-98.vercel.app`)
   - `JWT_SECRET` - A strong, random secret of at least 32 characters (the server exits fatally if empty or weak)
   - `SQLITE_PATH` - Path to database file (e.g. `/data/appointments.db` on persistent disk)
   - `DEFAULT_PLATFORM_ADMIN_EMAIL` & `DEFAULT_PLATFORM_ADMIN_PASSWORD` - (Optional) Initial platform admin credentials
3. **Mount a Persistent Disk on Render**:
   - In your Render Web Service dashboard, navigate to **Disks** -> **Add a Disk**.
   - Set **Name**: `sqlite-data`.
   - Set **Mount Path**: `/data`.
   - Set **Size**: 1 GB (or as needed).
   - Ensure `SQLITE_PATH` is set to `/data/appointments.db`.
   - This ensures your SQLite database and WAL logs persist across service deploys and restarts. Without a persistent disk mount at `/data`, ephemeral container restarts will reset database data.

### Deploying to Vercel (Frontend)

1. Connect your GitHub repository to Vercel
2. Set the following environment variable in Vercel dashboard:
   - `NEXT_PUBLIC_API_URL` - Your Render backend URL (e.g., `https://ai-chatbot-1vkx.onrender.com`)
3. Vercel will automatically detect Next.js and build/deploy

**Important**: Make sure the `FRONTEND_URL` in your backend matches your Vercel URL exactly, otherwise you'll get CORS errors.

## Initial Setup & Credentials

In `APP_ENV=development` mode, the database automatically seeds a `default` clinic tenant and development accounts for local testing. In `APP_ENV=production`, no hardcoded accounts or sample data are seeded; clinics register via `/signup` or platform admins are initialized via environment variables.

## API Endpoints (Multitenant)

All tenant-aware endpoints are prefixed with `/t/{tenant}` where `{tenant}` is a slug like `default`, `clinic-a`, etc.

| Method | Endpoint | Description |
|---|---|---|
| POST | `/t/{tenant}/chat` | AI booking handler |
| POST | `/t/{tenant}/login` | Admin login |
| GET | `/t/{tenant}/admin/appointments` | List appointments |
| POST | `/t/{tenant}/admin/appointments` | Create appointment |
| PUT | `/t/{tenant}/admin/appointments/:id` | Update appointment |
| DELETE | `/t/{tenant}/admin/appointments/:id` | Delete appointment |

## Example Chat Request

The chatbot supports natural language booking with conversation memory:

```bash
# Single message booking (default tenant)
curl -X POST http://localhost:8080/t/default/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Book me with Dr. Kim for 3 nov at 11am", "session_id": "user123"}'

# Multi-turn conversation (session_id maintains context)
curl -X POST http://localhost:8080/t/default/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Kevin leitich, i would like to see Wangechi", "session_id": "user123"}'

curl -X POST http://localhost:8080/t/default/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "3 nov at 4pm", "session_id": "user123"}'
```

## Expected Response

**Partial information** (asking for missing details):
```json
{
  "reply": "What date would you like to schedule for?"
}
```

**Complete booking**:
```json
{
  "message": "Perfect! I've booked your appointment with Dr. Kim on 2025-11-03 at 16:00 for checkup. Thank you, Kevin Leitich!",
  "appointment": {
    "patient_name": "Kevin Leitich",
    "doctor": "Dr. Kim",
    "date": "2025-11-03",
    "time": "16:00",
    "reason": "checkup",
    "status": "pending"
  }
}
```

## Features Details

### Intelligent Extraction
- **Names**: "Kevin leitich, i want..." → extracts "Kevin Leitich"
- **Doctors**: "i would like to see Wangechi" → extracts "Dr. Wangechi"
- **Dates**: "3 nov", "tomorrow", "november 4th" → converts to YYYY-MM-DD
- **Times**: "4pm", "2:30pm", "14:30" → converts to 24-hour HH:MM format
- **Reasons**: "for checkup", "dentist", "consultation" → extracts reason

### Conversation State
- Remembers all information across messages using `session_id`
- Never asks for information already provided
- Completes booking automatically when all required fields are collected

## Common Issues

| Problem | Fix |
|---|---|
| "Sorry, I couldn't reach the Groq API" | Set `GROQ_API_KEY` in `.env` file or export as environment variable |
| "Model decommissioned" error | Update `GROQ_MODEL` in `.env` to a current model (e.g., `llama-3.3-70b-versatile`) |
| CORS error | Fiber CORS middleware is enabled for dev |
| Token expired | Re-login to regenerate JWT |
| Bot asking for info already provided | Ensure `session_id` is consistent across requests |

## Available Groq Models

You can configure the model via `GROQ_MODEL` environment variable. Some options:
- `llama-3.3-70b-versatile` (default, recommended)
- `llama-3.1-8b-instant` (faster, smaller)
- `mixtral-8x7b-32768` (alternative)

## References
- [Groq API Docs](https://console.groq.com/docs)
- [Fiber Docs](https://docs.gofiber.io/)
- [React Docs](https://react.dev/)
- [GORM Docs](https://gorm.io/)

