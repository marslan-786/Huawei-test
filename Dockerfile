# --------------------------------------------------------
# Stage 1: Builder (Download Latest & Compile)
# --------------------------------------------------------
FROM golang:1.23-bookworm AS builder

# 1. System Dependencies (OpenCV & Build Tools)
# Yeh wo libraries hain jo GoCV aur Image Processing k liye zaroori hain
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

# 2. Copy Source Code ONLY (No go.mod needed locally)
COPY main.go .
COPY templates ./templates

# 3. Dynamic Dependency Management (Jadu yahan hai)
# Pehle purani mod files (agar ghalti se agayi hon) delete karein, phir naya init karein
RUN rm -f go.mod go.sum
RUN go mod init huawei-bot

# 4. Install HEAVY Libraries (LATEST Versions)
# Docker ab khud internet se latest version uthaye ga
RUN go get -u github.com/chromedp/chromedp@latest
RUN go get -u github.com/chromedp/cdproto@latest
RUN go get -u github.com/gin-gonic/gin@latest
RUN go get -u gocv.io/x/gocv@latest

# Tidy up to ensure everything matches
RUN go mod tidy

# 5. Build the App (CGO Enabled for OpenCV)
RUN CGO_ENABLED=1 GOOS=linux go build -a -o huawei-bot main.go

# --------------------------------------------------------
# Stage 2: Runner (Production Environment)
# --------------------------------------------------------
FROM debian:bookworm-slim

# 1. Runtime Dependencies (Browser + Graphics + OpenCV Runtime)
# Railway server par Chrome aur OpenCV chalane k liye zaroori cheezein
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
