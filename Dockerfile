# Stage 1: Builder
FROM golang:bookworm AS builder

# Basic Tools
RUN apt-get update && apt-get install -y \
    build-essential cmake git pkg-config \
    libopencv-dev libtesseract-dev libleptonica-dev tesseract-ocr \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Code Copy & Build
COPY main.go .
COPY templates ./templates
RUN rm -f go.mod go.sum
RUN go mod init huawei-bot
RUN go get -u github.com/chromedp/chromedp@latest
RUN go get -u github.com/chromedp/cdproto@latest
RUN go get -u github.com/gin-gonic/gin@latest
RUN go mod tidy
RUN go build -o huawei-bot main.go

# --------------------------------------------------------
# Stage 2: Runner (Production)
FROM debian:bookworm-slim

# Install Chromium + FFmpeg (Video k liye)
RUN apt-get update && apt-get install -y \
    ca-certificates \
    chromium \
    chromium-driver \
    fonts-liberation \
    fonts-noto-color-emoji \
    fonts-freefont-ttf \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Binary & Templates Copy
COPY --from=builder /app/huawei-bot .
COPY --from=builder /app/templates ./templates

# Captures Folder Permission
RUN mkdir captures && chmod 777 captures

EXPOSE 8080

# Auto-Start Command
CMD ["./huawei-bot"]
