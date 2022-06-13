FROM golang:1.18.3-alpine

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

EXPOSE 8000

ENV RUN_ADDRESS=:8000
ENV ACCRUAL_SYSTEM_ADDRESS=:8080
ENV DATABASE_URI=123

CMD ["/bin/bash"]