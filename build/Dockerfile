FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o server ./main
RUN chmod +x server

EXPOSE 3140
EXPOSE 3141

CMD ["./server", "-c", "./config/config.yaml"]