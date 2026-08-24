# Build multi-stage: o binario final nao carrega toolchain do Go nem
# codigo fonte, so o executavel estatico e certificados de CA para o proxy
# conseguir falar com upstreams https.
FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/gateway ./cmd/gateway
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build /out/gateway /app/gateway
COPY --from=build /out/seed /app/seed

EXPOSE 8080

ENTRYPOINT ["/app/gateway"]
