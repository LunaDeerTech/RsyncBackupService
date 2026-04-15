FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS backend
WORKDIR /app
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /rbs ./cmd/server/main.go

FROM alpine:3.19
RUN apk add --no-cache rsync openssh-client ca-certificates tzdata libnss_wrapper
COPY --from=backend /rbs /usr/local/bin/rbs
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
EXPOSE 8080
VOLUME ["/data"]
ENV RBS_DATA_DIR=/data
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["rbs"]