FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -v \
    -ldflags "-X github.com/Priyasharma620064/kubedriftguard/internal/version.Version=${VERSION} \
              -X github.com/Priyasharma620064/kubedriftguard/internal/version.GitCommit=${GIT_COMMIT} \
              -X github.com/Priyasharma620064/kubedriftguard/internal/version.BuildDate=${BUILD_DATE}" \
    -o /kubedriftguard ./cmd/controller/

# Runtime image
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /kubedriftguard /kubedriftguard
COPY AGENTS.md /AGENTS.md

USER 65532:65532

ENTRYPOINT ["/kubedriftguard"]
CMD ["controller"]
