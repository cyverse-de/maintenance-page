FROM golang:1.25.3-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

ARG TARGETOS=linux
ARG TARGETARCH=amd64

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY public ./public

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/maintenance-page ./cmd/maintenance-page

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/maintenance-page /app/maintenance-page
COPY --from=builder /src/public /app/public

EXPOSE 8080 8081

ENTRYPOINT ["/app/maintenance-page"]
