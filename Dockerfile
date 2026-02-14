FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /bin/picoclaw ./cmd/picoclaw

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S picoclaw && adduser -S picoclaw -G picoclaw

COPY --from=builder /bin/picoclaw /usr/local/bin/picoclaw

RUN mkdir -p /home/picoclaw/.picoclaw/workspace/memory \
             /home/picoclaw/.picoclaw/workspace/skills \
             /home/picoclaw/.picoclaw/workspace/sessions \
             /home/picoclaw/.picoclaw/workspace/cron \
             /home/picoclaw/.picoclaw/workspace/stm \
    && chown -R picoclaw:picoclaw /home/picoclaw/.picoclaw

COPY --chown=picoclaw:picoclaw skills/ /home/picoclaw/.picoclaw/workspace/skills/

USER picoclaw

EXPOSE 18790

ENTRYPOINT ["picoclaw"]
CMD ["gateway"]
