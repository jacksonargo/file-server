FROM golang:1.16 as builder
WORKDIR /go/src/app/
ENV CGO_ENABLED=0 GOOS=linux GO111MODULE=on
COPY go.mod go.sum main.go main_test.go ./
RUN go test -v -cover ./...
RUN go build -o file-server

FROM scratch as app
COPY --from=builder /go/src/app/file-server /bin/
EXPOSE 8080
VOLUME /srv
ENV FILE_SERVER_LISTEN_ADDRESS=0.0.0.0:8080 FILE_SERVER_CONTENT_ROOT=/srv
ENTRYPOINT ["/bin/file-server"]
