# (Previous Go setup...)
FROM golang:bookworm AS builder
# ... (Install dependencies and build app like before) ...
WORKDIR /app
COPY main.go .
COPY templates ./templates
RUN rm -f go.mod go.sum
RUN go mod init huawei-bot
RUN go get -u github.com/chromedp/chromedp@latest
RUN go get -u github.com/chromedp/cdproto@latest
RUN go get -u github.com/gin-gonic/gin@latest
# gocv abhi use nahi kar rahe to hata diya, simple rakha hai
RUN go mod tidy
RUN go build -o huawei-bot main.go

# --------------------------------------------------------
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    chromium \
    chromium-driver \
    fonts-liberation \
    fonts-noto-color-emoji \
    fonts-freefont-ttf \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Folders Copy
COPY --from=builder /app/huawei-bot .
COPY --from=builder /app/templates ./templates

# Captures folder create karein
RUN mkdir captures && chmod 777 captures

EXPOSE 8080

CMD ["./huawei-bot"]
