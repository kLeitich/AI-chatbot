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
| `PORT` | 8080 | Server port |
| `JWT_SECRET` | supersecret | Secret for JWT token signing |
| `GROQ_API_KEY` | **required** | Groq API key ([get one here](https://console.groq.com/)) |
| `GROQ_MODEL` | llama-3.3-70b-versatile | Groq model to use |
| `SQLITE_PATH` | appointments.db | SQLite database file path |
| `DEFAULT_ADMIN_EMAIL` | admin@example.com | Default admin email |
| `DEFAULT_ADMIN_PASSWORD` | admin123 | Default admin password |

## Features

### Intelligent Appointment Booking
- **Conversation State Management**: Tracks appointment details across multiple messages using session IDs
- **Smart Extraction**: Automatically extracts doctor, date, time, patient name, and reason from natural language
- **Context Awareness**: Never asks for information already provided
- **Time Normalization**: Automatically converts "4pm" → "16:00", "2:30pm" → "14:30"
- **Name Extraction**: Handles patterns like "Kevin Leitich, i want to see..." or "my name is..."
- **Doctor Extraction**: Handles "Dr. Kim", "doctor Kim", "i want to see Wangechi"

### API Endpoints

| Method | Endpoint | Description |
|---|---|---|
| GET | `/health` | Health check |
| POST | `/chat` | AI-powered chat booking (requires `message` and optional `session_id`) |
| POST | `/register` | User registration |
| POST | `/login` | Admin/user login |
| GET | `/admin/appointments` | List all appointments (requires JWT) |
| POST | `/admin/appointments` | Create appointment manually (requires JWT) |
| PUT | `/admin/appointments/:id` | Update appointment (requires JWT) |
| DELETE | `/admin/appointments/:id` | Delete appointment (requires JWT) |

### Chat Endpoint Details

**Request**:
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
