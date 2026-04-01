FROM golang:1.22-alpine AS go-builder
WORKDIR /workspace

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN mkdir -p /out && go build -o /out/rsync-backup-service ./cmd/server

FROM golang:1.22-alpine AS runtime
WORKDIR /app

RUN apk add --no-cache ca-certificates rsync openssh-client \
	&& addgroup -S rbs && adduser -S -G rbs rbs \
	&& mkdir -p /var/lib/rsync-backup-service/data /app/web \
	&& chown -R rbs:rbs /app /var/lib/rsync-backup-service

COPY --from=go-builder /out/rsync-backup-service ./rsync-backup-service

ENV RBS_PORT=8080 \
	RBS_DATA_DIR=/var/lib/rsync-backup-service/data \
	RBS_JWT_SECRET=change-me \
	RBS_ADMIN_USER=admin \
	RBS_ADMIN_PASSWORD=change-me

VOLUME ["/var/lib/rsync-backup-service/data"]
EXPOSE 8080

USER rbs

CMD ["./rsync-backup-service"]