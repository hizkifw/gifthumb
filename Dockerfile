FROM golang:1.20-alpine

WORKDIR /app

RUN apk update && \
    apk add --no-cache git ffmpeg && \
    git config --global --add safe.directory /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o gifthumb

CMD ["./gifthumb"]