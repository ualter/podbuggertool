FROM golang
RUN mkdir -p /go/src/podbuggertool
ADD . ~/go/src/podbuggertool/podbuggertool
WORKDIR /go
RUN go get ./...
RUN go install -v ./...
CMD ["/go/bin/podbuggertool"]