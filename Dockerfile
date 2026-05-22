FROM golang:1.22-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ARG TARGET=api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${TARGET}

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/app /app/app

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/app/app"]
