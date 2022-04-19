FROM golang:alpine as builder

RUN apk add --no-cache git
WORKDIR /scraper-src
COPY . /scraper-src
RUN go mod download && \
    go build -o /scraper .

FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /scraper /
ENTRYPOINT ["/scraper"]
