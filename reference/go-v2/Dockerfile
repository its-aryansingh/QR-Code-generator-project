# syntax=docker/dockerfile:1
ARG CMD=api

FROM golang:1.25-bookworm AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG CMD
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /bin/app ./cmd/${CMD}

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /bin/app /app/server

USER nonroot:nonroot

ENTRYPOINT ["/app/server"]
