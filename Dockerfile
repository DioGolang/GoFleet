FROM golang:1.25.5 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE_NAME

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server cmd/${SERVICE_NAME}/main.go

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache netcat-openbsd ca-certificates tzdata

COPY --from=builder /app/server .
COPY --from=builder /app/.env .

CMD ["./server"]