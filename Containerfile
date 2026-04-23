# Stage 1: Build SvelteKit frontend
FROM node:22-alpine AS frontend
RUN corepack enable && corepack prepare pnpm@latest --activate
WORKDIR /app
COPY sveltekit/package.json sveltekit/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY sveltekit/ ./
RUN pnpm build

# Stage 2: Build Go backend
FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Stage 3: Runtime
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /server ./server
COPY --from=frontend /pb_public ./pb_public/
EXPOSE 8090
VOLUME /app/pb_data
CMD ["./server", "serve", "--http=0.0.0.0:8090"]
