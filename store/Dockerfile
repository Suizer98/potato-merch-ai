# syntax=docker/dockerfile:1

FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:22-alpine AS runtime
WORKDIR /app
COPY package.json ./
COPY server.mjs crm-client.mjs load-env.mjs api-handlers.mjs ./
COPY --from=build /app/dist ./dist
ENV NODE_ENV=production
ENV PORT=3001
ENV CRM_URL=http://server:3000
ENV SERVER_URL=http://localhost:3000
ENV ADMIN_EMAIL=admin@example.com
ENV ADMIN_PASSWORD=admin123
EXPOSE 3001
HEALTHCHECK --interval=15s --timeout=5s --retries=10 --start-period=10s \
  CMD node -e "fetch('http://127.0.0.1:3001/api/health').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"
CMD ["node", "server.mjs"]
