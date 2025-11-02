# # syntax=docker/dockerfile:1

# FROM node:18-alpine AS deps
# WORKDIR /app
# COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* ./
# RUN npm install || true

# FROM node:18-alpine AS builder
# WORKDIR /app
# COPY --from=deps /app/node_modules ./node_modules
# COPY . .
# RUN npm run build

# FROM node:18-alpine AS runner
# WORKDIR /app
# ENV NODE_ENV=production
# COPY --from=builder /app .
# EXPOSE 3000
# CMD ["npm", "start"]
# frontend/Dockerfile
FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build
EXPOSE 3000
CMD ["npm", "start"]
