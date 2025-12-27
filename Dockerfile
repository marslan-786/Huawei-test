# --------------------------------------------------------
# Stage 1: Builder (Download Latest & Compile)
# --------------------------------------------------------
# FIX: Hum ne specific version k bajaye 'bookworm' use kiya hai
# taake ye hamesha LATEST Go version (e.g 1.25+) uthaye.
FROM golang:bookworm AS builder

# 1. System Dependencies (OpenCV & Build Tools)
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    git \
    pkg-config \
    libopencv-dev \ 
    libtesseract-dev \
    libleptonica-dev \
    tesseract-ocr \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 2. Copy Source Code ONLY
COPY main.go .
COPY templates ./templates

# 3. Dynamic Dependency Management
RUN rm -f go.mod go.sum
RUN go mod init huawei-bot

# 4. Install HEAVY Libraries (LATEST Versions)
# Ab ye error nahi dega kyun k Base Image latest hai
RUN go get -u github.com/chromedp/chromedp@latest
RUN go get -u github.com/chromedp/cdproto@latest
RUN go get -u github.com/gin-gonic/gin@latest
RUN go get -u gocv.io/x/gocv@latest

# Tidy up
RUN go mod tidy

# 5. Build the App (CGO Enabled for OpenCV)
RUN CGO_ENABLED=1 GOOS=linux go build -a -o huawei-bot main.go

# --------------------------------------------------------
# Stage 2: Runner (Production Environment)
# --------------------------------------------------------
FROM debian:bookworm-slim

# 1. Runtime Dependencies (Browser + Graphics + OpenCV Runtime)
RUN apt-get update && apt-get install -y \
    ca-certificates \
    chromium \
    chromium-driver \
    libopencv-dev \
    libtesseract-dev \
    tesseract-ocr \
    fonts-liberation \
    fonts-noto-color-emoji \
    fonts-freefont-ttf \
    xvfb \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 2. Copy Binary from Builder
COPY --from=builder /app/huawei-bot .
COPY --from=builder /app/templates ./templates

# 3. Port Expose
EXPOSE 8080

# 4. Run Command
CMD ["./huawei-bot"]
