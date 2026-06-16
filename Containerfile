# Stage 1: Build SvelteKit frontend
FROM node:22-alpine AS frontend
RUN corepack enable && corepack prepare pnpm@11 --activate
WORKDIR /app
# Vite resolves $env/static/public at build time. The runtime value comes
# from the orchestrator's .env, so any non-empty default works here.
ENV PUBLIC_PB_PORT=8090
ARG VERSION=dev
ENV PUBLIC_APP_VERSION=${VERSION}
COPY sveltekit/package.json sveltekit/pnpm-lock.yaml sveltekit/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY sveltekit/ ./
RUN pnpm build

# Stage 2: Build Go backend
FROM golang:1.25-alpine AS backend
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X github.com/Stewball32/xemu-cartographer/internal/version.Version=${VERSION} -X github.com/Stewball32/xemu-cartographer/internal/version.Commit=${COMMIT} -X github.com/Stewball32/xemu-cartographer/internal/version.Date=${DATE}" -o /server ./cmd/server

# Stage 3: Runtime
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /server ./server
COPY --from=frontend /pb_public ./pb_public/
EXPOSE 8090
VOLUME /app/pb_data
CMD ["./server", "serve", "--http=0.0.0.0:8090"]
