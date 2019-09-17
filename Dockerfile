FROM golang:alpine AS builder

WORKDIR $GOPATH/src/remote-pc-server/
COPY . .
RUN go build -o /go/bin/server



FROM alpine

COPY --from=builder /go/bin/server /go/bin/server
CMD [ "/go/bin/server" ]