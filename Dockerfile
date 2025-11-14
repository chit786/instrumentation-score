FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/instrumentation-score-service .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -g 1001 -S instrumentation && \
    adduser -u 1001 -S instrumentation -G instrumentation

WORKDIR /app

COPY --from=builder /app/bin/instrumentation-score-service ./instrumentation-score-service
COPY rules_config.yaml ./rules_config.yaml

RUN chown -R instrumentation:instrumentation /app

USER instrumentation

ENTRYPOINT ["./instrumentation-score-service"]
