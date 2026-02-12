FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /bin/picoclaw ./cmd/picoclaw

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/picoclaw /usr/local/bin/picoclaw

RUN mkdir -p /root/.picoclaw/workspace/memory \
             /root/.picoclaw/workspace/skills

COPY skills/ /root/.picoclaw/workspace/skills/

EXPOSE 18790

ENTRYPOINT ["picoclaw"]
CMD ["gateway"]
