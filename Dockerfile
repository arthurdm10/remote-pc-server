FROM golang:alpine AS builder

WORKDIR $GOPATH/src/remote-pc-server/
COPY . .
RUN apk add --update && apk add git
RUN go get 
RUN go build -o /go/bin/server



FROM alpine

COPY --from=builder /go/bin/server /go/bin/server
CMD [ "/go/bin/server" ]