FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ecommerce-ops ./cmd/ecommerce-ops
COPY internal/backups ./internal/backups
COPY internal/operability ./internal/operability
COPY internal/schemacontract ./internal/schemacontract
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ecommerce-ops ./cmd/ecommerce-ops

FROM alpine:3.23

RUN apk add --no-cache ca-certificates postgresql-client tzdata \
    && addgroup -S -g 65532 ecommerce \
    && adduser -S -D -H -u 65532 -G ecommerce ecommerce
COPY --from=build /out/ecommerce-ops /usr/local/bin/ecommerce-ops
USER ecommerce:ecommerce
ENTRYPOINT ["/usr/local/bin/ecommerce-ops"]
