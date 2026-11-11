# Backend (Go + Fiber + Groq API)

## Run

```bash
cd backend
cp env.example .env
# Edit .env and add your GROQ_API_KEY
go mod tidy
go run .
```

The backend automatically loads environment variables from `.env` file if it exists.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | Environment mode (`development` or `production`) |
| `PORT` | 8080 | Server port |
| `JWT_SECRET` | Required in prod (min 32 chars) | Secret for JWT token signing (refuses to start with weak/empty secret in prod) |
| `TRUST_PROXY` | `false` | Set to `true` when behind a reverse proxy (e.g. Render) to enable accurate client IP rate limiting |
| `GROQ_API_KEY` | **required** | Groq API key ([get one here](https://console.groq.com/)) |
| `GROQ_MODEL` | `llama-3.3-70b-versatile` | Groq model to use |
| `SQLITE_PATH` | `appointments.db` | SQLite database file path (use `/data/appointments.db` in Docker/Render) |
| `DEFAULT_ADMIN_EMAIL` | `admin@example.com` (dev only) | Optional seed admin email |
| `DEFAULT_ADMIN_PASSWORD` | `admin123` (dev only) | Optional seed admin password |

## Docker and Persistence

The Docker image runs as an unprivileged user (`appuser`) and exposes a persistent volume mount point at `/data`.

### Local Docker Compose
When running via `docker-compose up`, data is stored in the named volume `backend_data` mounted at `/data`, persisting SQLite data across container recreation.

### Render Persistent Disk
When deploying the backend container to Render:
1. Attach a persistent disk in the Render service settings (under **Disks** -> **Add a Disk**).
2. Set Mount Path to `/data`.
3. Set environment variable `SQLITE_PATH=/data/appointments.db`.
4. Ensure `TRUST_PROXY=true` so rate limiting properly resolves client IPs from `X-Forwarded-For`.

## Features

### Intelligent Appointment Booking
- **Conversation State Management**: Tracks appointment details across multiple messages using session IDs
- **Smart Extraction**: Automatically extracts doctor, date, time, patient name, and reason from natural language
- **Context Awareness**: Never asks for information already provided
- **Time Normalization**: Automatically converts "4pm" → "16:00", "2:30pm" → "14:30"
- **Name Extraction**: Handles patterns like "Kevin Leitich, i want to see..." or "my name is..."
- **Doctor Extraction**: Handles "Dr. Kim", "doctor Kim", "i want to see Wangechi"

### API Endpoints (Multitenant)

All tenant-aware endpoints are prefixed with `/t/{tenant}` where `{tenant}` is a slug like `default`, `clinic-a`, etc.

| Method | Endpoint | Description |
|---|---|---|
| GET | `/health` | Health check |
| POST | `/t/{tenant}/chat` | AI-powered chat booking (requires `message` and optional `session_id`) |
| POST | `/t/{tenant}/register` | User registration for that tenant |
| POST | `/t/{tenant}/login` | Admin/user login for that tenant |
| GET | `/t/{tenant}/admin/appointments` | List all appointments (requires JWT) |
| POST | `/t/{tenant}/admin/appointments` | Create appointment manually (requires JWT) |
| PUT | `/t/{tenant}/admin/appointments/:id` | Update appointment (requires JWT) |
| DELETE | `/t/{tenant}/admin/appointments/:id` | Delete appointment (requires JWT) |

### Chat Endpoint Details

**Request (default tenant)**:
```json
{
  "message": "Book me with Dr. Kim for 3 nov at 4pm",
  "session_id": "optional-session-id-for-conversation-memory"
}
```

**Response (partial info)**:
```json
{
  "reply": "What's your name, please?"
}
```

**Response (complete booking)**:
```json
{
  "message": "Perfect! I've booked your appointment...",
  "appointment": {
    "id": 1,
    "patient_name": "Kevin Leitich",
    "doctor": "Dr. Kim",
    "date": "2025-11-03",
    "time": "16:00",
    "reason": "checkup",
    "status": "pending"
  }
}
```

## Development

The backend uses:
- **Fiber v2** for HTTP server
- **GORM** for database ORM
- **SQLite** for data storage
- **Groq API** for AI chat completions
- **JWT** for authentication
