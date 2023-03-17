FROM golang:1.16-alpine

ENV TOKEN='<Add Token>'

COPY . /webapp

WORKDIR /webapp

RUN go build -o main .

CMD ["./main"]