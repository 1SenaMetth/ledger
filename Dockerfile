# syntax=docker/dockerfile:1

# --- build ---
FROM golang:1.27-alpine AS build

WORKDIR /src

# Copy manifests first so dependency downloads are cached independently of
# source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none

# CGO_ENABLED=0 produces a static binary, which is what lets the final stage be
# a distroless image with no libc at all.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/api ./cmd/api

# --- runtime ---
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /api

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/api"]
