FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# TAGS управляет встроенными драйверами БД.
# Для SQLite:   docker build --build-arg TAGS=sqlite .
# Для Postgres: docker build --build-arg TAGS=postgres .  (по умолчанию)
# Оба сразу:    docker build --build-arg TAGS="sqlite postgres" .
ARG TAGS=postgres
RUN CGO_ENABLED=0 go build -tags "${TAGS}" -trimpath -ldflags="-s -w" -o /marina ./cmd/marina

FROM scratch
COPY --from=builder /marina /marina
EXPOSE 8443
ENTRYPOINT ["/marina"]
