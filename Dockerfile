FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /plotbunni-backend .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /plotbunni-backend .
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s \
    CMD wget -qO- http://localhost:8080/health || exit 1
CMD ["./plotbunni-backend"]
