# AI-Powered Doctor Appointment Chatbot

A Go + React full-stack chatbot integrated with Groq API to handle natural language appointment scheduling with intelligent conversation state management.

## Features
- Natural chat-driven booking with conversational memory
- AI-driven extraction of doctor, date, time, patient name, and reason
- Smart conversation state tracking (never asks for information already provided)
- Automatic time format conversion (4pm → 16:00)
- Context-aware responses that remember previous messages
- Admin panel for appointment management (table + calendar views)
- Calendar dashboard to view and manage appointments (create/edit/delete)
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

**Frontend (.env)**: `VITE_API_URL` (default: http://localhost:8080)

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

### Frontend
```bash
cd frontend
npm install
npm run dev
```

## Docker Deployment
```bash
docker-compose up --build
```

Access:
- Frontend → http://localhost
- Backend → http://localhost:8080

**Note**: For Docker deployment, ensure `GROQ_API_KEY` is set in the backend service environment variables.

## Default Admin

| Email | Password |
|---|---|
| admin@example.com | admin123 |

## API Endpoints

| Method | Endpoint | Description |
|---|---|---|
| POST | /chat | AI booking handler |
| POST | /login | Admin login |
| GET | /admin/appointments | List appointments |
| POST | /admin/appointments | Create appointment |
| PUT | /admin/appointments/:id | Update |
| DELETE | /admin/appointments/:id | Delete |

## Example Chat Request

The chatbot supports natural language booking with conversation memory:

```bash
# Single message booking
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Book me with Dr. Kim for 3 nov at 11am", "session_id": "user123"}'

# Multi-turn conversation (session_id maintains context)
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Kevin leitich, i would like to see Wangechi", "session_id": "user123"}'

curl -X POST http://localhost:8080/chat \
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

