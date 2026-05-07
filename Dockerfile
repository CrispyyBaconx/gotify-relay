FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gotify-relay ./cmd/gotify-relay \
    && mkdir -p /out/data

FROM alpine:3.21

RUN apk add --no-cache curl && mkdir -p /data /etc/gotify-relay
COPY --from=build /out/gotify-relay /gotify-relay
EXPOSE 8080

ENTRYPOINT ["/gotify-relay"]
CMD ["-config", "/etc/gotify-relay/config.yaml"]
