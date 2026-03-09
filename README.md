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
- `PORT` (default: 8080)
- `JWT_SECRET` (default: supersecret)
- `GROQ_API_KEY` (**required** - get from [Groq Console](https://console.groq.com/))
- `GROQ_MODEL` (default: llama-3.3-70b-versatile)
- `SQLITE_PATH` (default: appointments.db)
- `DEFAULT_ADMIN_EMAIL` (default: admin@example.com)
- `DEFAULT_ADMIN_PASSWORD` (default: admin123)
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
   - `PORT=8080` (Render will override this automatically)
   - `GROQ_API_KEY` - Your Groq API key
   - `FRONTEND_URL` - Your Vercel frontend URL (e.g., `https://ai-chatbot-gamma-blue-98.vercel.app`)
   - `JWT_SECRET` - A secure secret for JWT tokens
   - `SQLITE_PATH` - Path to database file (default: `appointments.db`)

### Deploying to Vercel (Frontend)

1. Connect your GitHub repository to Vercel
2. Set the following environment variable in Vercel dashboard:
   - `NEXT_PUBLIC_API_URL` - Your Render backend URL (e.g., `https://ai-chatbot-1vkx.onrender.com`)
3. Vercel will automatically detect Next.js and build/deploy

**Important**: Make sure the `FRONTEND_URL` in your backend matches your Vercel URL exactly, otherwise you'll get CORS errors.

## Default Admin

| Email | Password |
|---|---|
| admin@example.com | admin123 |

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

