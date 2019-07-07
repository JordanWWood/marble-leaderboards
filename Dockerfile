FROM golang:alpine as builder
ENV GO111MODULE=on
RUN apk add git
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN go build -o main .
FROM alpine
RUN adduser -S -D -H -h /app appuser
USER appuser
COPY --from=builder /build/main /app/
WORKDIR /app
CMD ["./main"]