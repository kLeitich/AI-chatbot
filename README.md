# AI-Powered Doctor Appointment Chatbot

A full-stack AI chatbot for scheduling doctor appointments, built with Go (Fiber) for the backend and Next.js (React) for the frontend, using Ollama for natural language processing.

## Features
- **Natural Language Booking**: Schedule appointments through a simple chat interface.
- **AI-Powered NLP**: Uses a local Ollama model (e.g., Phi-3, Llama3) to understand and extract appointment details from user messages.
- **Admin Dashboard**: A comprehensive dashboard for staff to manage appointments.
- **Dual-View Interface**: View appointments in a filterable table and weekly/monthly view.
- **CRUD Operations**: Admins can create, read, update, and delete appointments.
- **Dockerized**: Fully containerized with Docker Compose for easy setup and deployment.
- **Persistent State**: Uses SQLite for data storage and JWT for secure admin sessions.

## Tech Stack

| Area      | Technology                               |
|-----------|------------------------------------------|
| **Backend**   | Go, Fiber, GORM, SQLite, JWT             |
| **Frontend**  | Next.js, React, Tailwind CSS, Axios      |
| **AI/NLP**    | Ollama (self-hosted)                     |
| **DevOps**    | Docker, Docker Compose, Nginx            |

## System Requirements

| Component | Version |
|-----------|---------|
| Go        | 1.20+   |
| Node.js   | 18+     |
| Ollama    | Latest  |
| Docker    | 24+     |

## Environment Variables

Create `.env` files for each service by copying the example files:

```bash
# For the backend
cp backend/env.example backend/.env

# For the frontend
cp frontend/env.example frontend/.env.local
```

### Backend (`backend/.env`)
- `PORT`: Port for the Go server (e.g., `8080`).
- `JWT_SECRET`: Secret key for signing JWTs.
- `OLLAMA_MODEL`: The Ollama model to use (e.g., `phi3`).
- `SQLITE_PATH`: Path to the SQLite database file (e.g., `appointments.db`).
- `DEFAULT_ADMIN_EMAIL`: Default admin user email.
- `DEFAULT_ADMIN_PASSWORD`: Default admin user password.

### Frontend (`frontend/.env.local`)
- `NEXT_PUBLIC_API_URL`: The absolute URL of the backend API (e.g., `http://localhost:8080`).

## Local Development Setup

### 1. Run Ollama
First, ensure the Ollama service is running and you have pulled a model:
```bash
# Start the Ollama server in the background
ollama serve &

# Pull a model (phi3 is a good, small starting point)
ollama pull phi3
```

### 2. Run the Backend
```bash
cd backend
go mod tidy
go run .
```
The backend server will start on the port specified in your `.env` file (e.g., http://localhost:8080).

### 3. Run the Frontend
```bash
cd frontend
npm install
npm run dev
```
The Next.js frontend will be available at http://localhost:3000.

## Docker Deployment

To run the entire stack (Frontend, Backend, Nginx) using Docker:
```bash
# Make sure you have created the .env files first
docker-compose up --build
```

Access the application:
- **Chatbot & Admin Panel**: http://localhost
- **Backend API (if needed directly)**: http://localhost:8080

*Note: For Docker deployment, Ollama must be running on the host machine and accessible from the Docker containers.*

## Default Admin Credentials

| Email             | Password |
|-------------------|----------|
| `admin@example.com` | `admin123` |

These can be changed in the `backend/.env` file.

## API Endpoints

| Method   | Endpoint                  | Description                               | Auth      |
|----------|---------------------------|-------------------------------------------|-----------|
| `POST`   | `/chat`                   | Send a message to the chatbot for processing. | Public    |
| `POST`   | `/login`                  | Authenticate an admin user.               | Public    |
| `GET`    | `/admin/appointments`     | List all appointments.                    | JWT Token |
| `POST`   | `/admin/appointments`     | Create a new appointment.                 | JWT Token |
| `PUT`    | `/admin/appointments/:id` | Update an existing appointment.           | JWT Token |
| `DELETE` | `/admin/appointments/:id` | Delete an appointment.                    | JWT Token |

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

