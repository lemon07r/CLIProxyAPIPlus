FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for patch application
RUN apk add --no-cache git

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Apply patches in order (001, 002, 003, etc.)
# Patches are applied after sync with upstream to add custom features
RUN echo "=== Applying custom patches ===" && \
    if [ -d patches ] && [ "$(ls -A patches/*.patch 2>/dev/null)" ]; then \
        git init 2>/dev/null || true; \
        git config user.email "build@local" 2>/dev/null || true; \
        git config user.name "Build" 2>/dev/null || true; \
        git add -A 2>/dev/null || true; \
        git commit -m "pre-patch" 2>/dev/null || true; \
        for patch in $(ls patches/*.patch 2>/dev/null | sort); do \
            echo "Applying: $patch"; \
            if git apply --check "$patch" 2>/dev/null; then \
                git apply "$patch" && echo "  SUCCESS: $patch"; \
            else \
                echo "  SKIP: $patch (may already be applied or conflicts)"; \
            fi; \
        done; \
    else \
        echo "No patches to apply"; \
    fi && \
    echo "=== Patch application complete ==="

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X 'main.Version=${VERSION}-plus' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" -o ./CLIProxyAPIPlus ./cmd/server/

FROM alpine:3.22.0

RUN apk add --no-cache tzdata ca-certificates

RUN mkdir /CLIProxyAPI

COPY --from=builder ./app/CLIProxyAPIPlus /CLIProxyAPI/CLIProxyAPIPlus

COPY config.example.yaml /CLIProxyAPI/config.example.yaml

WORKDIR /CLIProxyAPI

EXPOSE 8317

ENV TZ=Asia/Shanghai

RUN cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo "${TZ}" > /etc/timezone

CMD ["./CLIProxyAPIPlus"]
