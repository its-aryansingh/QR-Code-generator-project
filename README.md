# QRit — QR Code Generator SaaS

A full-stack SaaS for generating, managing, and tracking QR codes. Built with Django REST Framework and Next.js.

![Python](https://img.shields.io/badge/Python-3776AB?style=for-the-badge&logo=python&logoColor=white)
![Django](https://img.shields.io/badge/Django-092E20?style=for-the-badge&logo=django&logoColor=white)
![Next.js](https://img.shields.io/badge/Next.js-000000?style=for-the-badge&logo=next.js&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-316192?style=for-the-badge&logo=postgresql&logoColor=white)

## Stack

- **Backend** — Django 4.2, Django REST Framework, PostgreSQL
- **Auth** — Custom JWT (PyJWT + bcrypt)
- **QR** — qrcode[pil]
- **Payments** — Stripe
- **Frontend** — Next.js 16, TypeScript, Tailwind CSS, ShadCN
- **State** — Zustand

## Getting Started

### With Docker

```bash
docker-compose up -d
```

App runs at http://localhost:3000, API at http://localhost:8084.

### Local Development

**Database:**
```bash
docker run -d --name qrapp-db \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=qrapp \
  -p 5432:5432 \
  postgres:16-alpine
```

**Backend:**
```bash
cd backend
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate
pip install -r requirements.txt
cp .env.example .env
python manage.py migrate
python manage.py runserver 0.0.0.0:8084
```

**Frontend:**
```bash
cd frontend
npm install
npm run dev
```

Frontend: http://localhost:3000 | API: http://localhost:8084

## Project Structure

```
qr_code_project/
├── backend/
│   ├── api/
│   │   ├── models.py
│   │   ├── urls.py
│   │   ├── views/
│   │   └── utils/
│   ├── qrapp/
│   │   └── settings.py
│   ├── manage.py
│   └── requirements.txt
├── frontend/
│   └── src/
│       ├── app/
│       ├── components/
│       ├── lib/
│       └── types/
├── docker-compose.yml
└── render.yaml
```

## Environment Variables

**Backend** (`backend/.env`):
```env
DATABASE_URL=postgresql://postgres:postgres@localhost:5432/qrapp
JWT_SECRET=your-secret
STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=
APP_BASE_URL=http://localhost:3000
API_BASE_URL=http://localhost:8084
```

**Frontend** (`frontend/.env.local`):
```env
NEXT_PUBLIC_API_URL=http://localhost:8084/api/v1
```

## API

Base URL: `/api/v1/`

| Method | Endpoint | Auth |
|--------|----------|------|
| POST | `/auth/register` | — |
| POST | `/auth/login` | — |
| POST | `/auth/refresh` | Refresh token |
| GET/PUT | `/auth/me` | JWT |
| POST | `/qr/generate` | JWT |
| GET | `/qr/history` | JWT |
| GET/PUT/DELETE | `/qr/<id>` | JWT |
| POST | `/public/generate` | — |
| GET | `/analytics/dashboard` | JWT |

## License

MIT
