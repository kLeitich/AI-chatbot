# AI-Powered Doctor Appointment Chatbot

A Go + React full-stack chatbot integrated with Ollama to handle natural language appointment scheduling.

## Features
- Natural chat-driven booking
- AI-driven date/time extraction
- Admin panel for appointment management
- Offline LLM integration (Ollama)
- Dockerized full stack

## System Requirements

| Component | Version |
|---|---|
| Go | 1.20+ |
| Node.js | 18+ |
| Ollama | Latest |
| Docker | 24+ |

## Installation

### Backend
```bash
cd backend
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

Ensure Ollama is running on host:
```bash
ollama serve &
ollama pull phi3
```

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
| PUT | /admin/appointments/:id | Update |
| DELETE | /admin/appointments/:id | Delete |

## Example Chat Request
```bash
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Book me an appointment with Dr. Kim tomorrow at 10am"}'
```

## Expected Response
```json
{
  "message": "Appointment booked",
  "appointment": {
    "patient_name": "John Doe",
    "doctor": "Dr. Kim",
    "date": "2025-10-28",
    "time": "10:00",
    "reason": "checkup",
    "status": "pending"
  }
}
```

## Common Issues

| Problem | Fix |
|---|---|
| Ollama not found | Install locally: `curl -fsSL https://ollama.com/install.sh` |
| JSON parse error | Update Ollama prompt to force JSON output |
| CORS error | Fiber CORS middleware is enabled for dev |
| Token expired | Re-login to regenerate JWT |

## References
- Ollama Docs
- Fiber Docs
- React Docs
- GORM Docs

## Development Notes (Cursor)
- Generate backend code first (Go Fiber API with routes, models, auth).
- Commit: `checkpoint: backend completed`
- Generate frontend (React + Tailwind + Vite).
- Commit: `checkpoint: frontend scaffolded`
- Generate Docker setup (backend + frontend + compose).
- Commit: `checkpoint: dockerized project`
- Generate READMEs.
- Commit: `checkpoint: docs added`
