# MedDesk AI Marketing Site (`website/`)

This folder contains a small static marketing site for the AI doctor appointment platform.

## What it is

- A single-page, responsive landing page at `website/index.html`
- Describes:
  - The AI chatbot that books doctor appointments
  - The multitenant backend (`/t/{tenant}/...` routes)
  - Admin dashboards and calendar views
  - Planned WhatsApp integration (via a webhook into `/t/{tenant}/chat`)

It is intentionally framework-free so you can host it anywhere as static HTML (e.g. Nginx, Vercel static export, S3, or your existing reverse proxy).

## Local preview

From the project root:

```bash
cd website
python -m http.server 4173
```

Then open:

- `http://localhost:4173/index.html`

## Structure

- `index.html` – The marketing page (hero, feature highlights, WhatsApp section)
- `README.md` – This file

You can extend this into a full marketing site or port the design into your existing Next.js frontend if you prefer everything under one framework.

