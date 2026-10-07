## Stage 1: build the frontend
FROM node:24-alpine AS frontend

RUN corepack enable
WORKDIR /app/frontend

COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm run build

## Stage 2: build the backend with the embedded frontend
FROM golang:1.26-alpine AS backend

ARG VERSION=dev
ARG COMMIT=unknown

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X github.com/filebrowser/filebrowser/v2/version.Version=${VERSION} -X github.com/filebrowser/filebrowser/v2/version.CommitSHA=${COMMIT}" \
    -o /filebrowser .

## Stage 3: fetch runtime dependencies
FROM alpine:3.23 AS fetcher

# install and copy ca-certificates, mailcap, and tini-static; download JSON.sh
RUN apk update && \
    apk --no-cache add ca-certificates mailcap tini-static && \
    wget -O /JSON.sh https://raw.githubusercontent.com/dominictarr/JSON.sh/0d5e5c77365f63809bf6e77ef44a1f34b0e05840/JSON.sh

## Stage 4: use lightweight BusyBox image for final runtime environment
FROM busybox:1.37.0-musl

# Define non-root user UID and GID
ENV UID=1000
ENV GID=1000

# Create user group and user
RUN addgroup -g $GID user && \
    adduser -D -u $UID -G user user

# Copy binary, scripts, and configurations into image with proper ownership
COPY --chown=user:user --from=backend /filebrowser /bin/filebrowser
COPY --chown=user:user docker/common/ /
COPY --chown=user:user docker/alpine/ /
COPY --chown=user:user --from=fetcher /sbin/tini-static /bin/tini
COPY --from=fetcher /JSON.sh /JSON.sh
COPY --from=fetcher /etc/ca-certificates.conf /etc/ca-certificates.conf
COPY --from=fetcher /etc/ca-certificates /etc/ca-certificates
COPY --from=fetcher /etc/mime.types /etc/mime.types
COPY --from=fetcher /etc/ssl /etc/ssl

# Create data directories, set ownership, and ensure healthcheck script is executable
RUN mkdir -p /config /database /srv && \
    chown -R user:user /config /database /srv \
    && chmod +x /healthcheck.sh

# Define healthcheck script
HEALTHCHECK --start-period=2s --interval=5s --timeout=3s CMD /healthcheck.sh

# Set the user, volumes and exposed ports
USER user

VOLUME /srv /config /database

EXPOSE 80

ENTRYPOINT [ "tini", "--", "/init.sh" ]
