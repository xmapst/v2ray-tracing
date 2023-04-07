FROM golang:alpine as builder

RUN apk add --no-cache git
WORKDIR /tracing-src
COPY tracing/* /tracing-src
RUN go mod download && \
    go build -o /tracing .

FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /tracing /
ENTRYPOINT ["/tracing"]
