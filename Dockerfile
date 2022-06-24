FROM golang:1.18.3-alpine

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

EXPOSE 8000

RUN go build -o gophermart ./cmd/gophermart

CMD ["/bin/bash"]