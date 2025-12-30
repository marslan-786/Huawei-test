FROM golang:bookworm AS builder
RUN apt-get update && apt-get install -y build-essential cmake git pkg-config libopencv-dev && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY main.go .
COPY templates ./templates
RUN rm -f go.mod go.sum
RUN go mod init huawei-bot
RUN go get -u github.com/chromedp/chromedp@latest
RUN go get -u github.com/chromedp/cdproto@latest
RUN go get -u github.com/gin-gonic/gin@latest
RUN go mod tidy
RUN go build -o huawei-bot main.go

FROM debian:bookworm-slim
# FFmpeg install karna zaroori hai
RUN apt-get update && apt-get install -y ca-certificates chromium chromium-driver ffmpeg && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/huawei-bot .
COPY --from=builder /app/templates ./templates
RUN mkdir captures && chmod 777 captures
EXPOSE 8080
CMD ["./huawei-bot"]