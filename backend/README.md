# Backend (Go + Fiber)

## Run

```bash
cd backend
go mod tidy
go run .
```

Env vars:
- `PORT` (default 8080)
- `JWT_SECRET` (default supersecret)
- `OLLAMA_MODEL` (default phi3)
- `SQLITE_PATH` (default appointments.db)
- `DEFAULT_ADMIN_EMAIL` / `DEFAULT_ADMIN_PASSWORD` (seed admin)

Endpoints:
- POST `/chat`
- POST `/login`, `/register`
- GET `/admin/appointments`
- PUT `/admin/appointments/:id`
- DELETE `/admin/appointments/:id`
