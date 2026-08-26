# Imagem unica para os dois servicos upstream de exemplo. O docker-compose
# escolhe qual binario rodar via "command", entao o build so acontece uma
# vez e as camadas ficam em cache entre os dois servicos.
FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/mock-billing ./upstreams/mock-billing
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/mock-inventory ./upstreams/mock-inventory

FROM alpine:3.20

WORKDIR /app
COPY --from=build /out/mock-billing /app/mock-billing
COPY --from=build /out/mock-inventory /app/mock-inventory
