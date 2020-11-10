FROM golang

RUN mkdir -p /go/src/podbuggertool
ADD . /go/src/podbuggertool
WORKDIR /go

RUN go get ./...
RUN go install -v ./...

CMD ["/go/bin/podbuggertool"]