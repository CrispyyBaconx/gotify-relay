FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gotify-relay ./cmd/gotify-relay \
    && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/gotify-relay /gotify-relay
COPY --from=build --chown=nonroot:nonroot /out/data /data
USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/gotify-relay"]
CMD ["-config", "/etc/gotify-relay/config.yaml"]
