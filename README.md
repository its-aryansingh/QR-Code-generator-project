# QRApp - Professional QR Code Generator

A high-performance, enterprise-grade SaaS for QR code generation built with **Go (Gin)** backend and **Next.js** frontend.

![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Next.js](https://img.shields.io/badge/Next.js-000000?style=for-the-badge&logo=next.js&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-316192?style=for-the-badge&logo=postgresql&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white)

## Features

- ⚡ **Lightning Fast** - Go backend with zero disk I/O for QR generation
- 🔒 **Secure** - JWT access/refresh token authentication with bcrypt
- 📊 **History Tracking** - View and manage all generated QR codes
- 🎨 **Premium UI** - Modern dark theme with Tailwind CSS + ShadCN
- 🐳 **Containerized** - Docker Compose for easy deployment

## Tech Stack

| Component | Technology |
|-----------|------------|
| **Backend** | Go 1.22+, Gin, GORM |
| **Database** | PostgreSQL 16 |
| **Auth** | JWT (Access/Refresh), bcrypt |
| **QR Engine** | github.com/skip2/go-qrcode |
| **Frontend** | Next.js 14, TypeScript, Tailwind CSS |
| **UI Components** | ShadCN UI |
| **State** | Zustand |
| **Container** | Docker, Docker Compose |

## Quick Start

### Prerequisites

- Docker & Docker Compose
- OR: Go 1.22+, Node.js 20+, PostgreSQL

### With Docker (Recommended)

```bash
# Clone and start all services
docker-compose up -d

# Access the app
open http://localhost:3000
```

### Manual Development

**1. Start PostgreSQL:**
```bash
docker run -d --name qrapp-db \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=qrapp \
  -p 5432:5432 \
  postgres:16-alpine
```

**2. Start Backend:**
```bash
cd backend
cp .env.example .env
go run ./cmd/server
```

**3. Start Frontend:**
```bash
cd frontend
npm install
npm run dev
```

**4. Access:**
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080

## API Endpoints

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `POST` | `/api/v1/auth/register` | Create new user | No |
| `POST` | `/api/v1/auth/login` | Login, returns JWT | No |
| `POST` | `/api/v1/auth/refresh` | Refresh access token | Refresh Token |
| `POST` | `/api/v1/qr/generate` | Generate QR code | JWT |
| `GET` | `/api/v1/qr/history` | Get user's QR history | JWT |
| `GET` | `/api/v1/qr/:id` | Get specific QR record | JWT |

## Project Structure

```
qr_code_project/
├── backend/
│   ├── cmd/server/           # Application entrypoint
│   ├── internal/
│   │   ├── config/           # Environment configuration
│   │   ├── database/         # GORM database connection
│   │   ├── handlers/         # HTTP handlers
│   │   ├── middleware/       # Auth, CORS middleware
│   │   ├── models/           # GORM models
│   │   ├── repository/       # Data access layer
│   │   └── services/         # Business logic
│   ├── pkg/utils/            # Shared utilities
│   ├── migrations/           # SQL migrations
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── app/              # Next.js App Router
│   │   ├── components/       # React components
│   │   ├── lib/              # API client, auth store
│   │   └── types/            # TypeScript types
│   └── Dockerfile
└── docker-compose.yml
```

## Environment Variables

### Backend (.env)
```env
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=qrapp
JWT_SECRET=your-secret-key
```

### Frontend (.env.local)
```env
NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1
```

## Running Tests

```bash
cd backend
go test ./... -v -cover
```

## License

MIT
