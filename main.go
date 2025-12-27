package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

//	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
)

// --- Global Variables ---
var (
	imgBuf []byte
	mu     sync.RWMutex
)

// --- SETTINGS (یہاں نمبر لکھیں) ---
const (
	// اپنا ورچول نمبر یہاں لکھیں (بغیر +92 کے)
	TargetPhoneNumber = "3177635849" 
	
	TargetURL = "https://id5.cloud.huawei.com/CAS/mobile/standard/register/wapRegister.html?reqClientType=7&loginChannel=7000000&regionCode=hk&loginUrl=https%3A%2F%2Fid5.cloud.huawei.com%2FCAS%2Fmobile%2Fstandard%2FwapLogin.html&lang=en-us&themeName=huawei#/wapRegister/regByPhone"
	Port      = ":8080"
)

func main() {
	go startWebServer()
	select {}
}

// --- Web Server Logic ---
func startWebServer() {
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.GET("/stream", func(c *gin.Context) {
		c.Header("Content-Type", "multipart/x-mixed-replace; boundary=frame")
		for {
			mu.RLock()
			data := imgBuf
			mu.RUnlock()
			if len(data) > 0 {
				c.Writer.Write([]byte(fmt.Sprintf("--frame\r\nContent-Type: image/jpeg\r\n\r\n%s\r\n", data)))
				c.Writer.Flush()
			}
			time.Sleep(100 * time.Millisecond)
		}
	})

	// بٹن دبانے پر یہ چلے گا
	r.POST("/start-bot", func(c *gin.Context) {
		// اب یہ فارم سے نہیں بلکہ اوپر والے Constant سے نمبر اٹھائے گا
		go runHuaweiBot(TargetPhoneNumber) 
		c.JSON(200, gin.H{"status": "Started", "number": TargetPhoneNumber})
	})

	r.Run(Port)
}

// --- Bot Logic ---
func runHuaweiBot(phoneNumber string) {
	log.Println("Testing Number:", phoneNumber)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Mobile Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Navigate(TargetURL),
		captureStream(ctx),
		chromedp.Sleep(3*time.Second),

		// 1. Country Select (Pakistan)
		chromedp.Click(`//div[contains(@class, 'hwid-input-div')]`, chromedp.NodeVisible),
		chromedp.Sleep(1*time.Second),
		chromedp.SendKeys(`input[type="search"]`, "Pakistan"),
		chromedp.Sleep(1*time.Second),
		captureStream(ctx),
		chromedp.Click(`//li[contains(text(), 'Pakistan')]`, chromedp.NodeVisible),
		chromedp.Sleep(1*time.Second),

		// 2. Input Number (From Script)
		chromedp.SendKeys(`input[type="tel"]`, phoneNumber),
		chromedp.Sleep(500*time.Millisecond),
		captureStream(ctx),

		// 3. Click Get Code
		chromedp.Click(`//div[contains(text(), 'Get code')]`, chromedp.NodeVisible),
		
		// 4. Watch Screen Loop
		chromedp.ActionFunc(func(c context.Context) error {
			for i := 0; i < 60; i++ {
				var buf []byte
				chromedp.CaptureScreenshot(&buf).Do(c)
				updateStream(buf)
				time.Sleep(500 * time.Millisecond)
			}
			return nil
		}),
	)

	if err != nil {
		log.Printf("Error: %v", err)
	}
}

func updateStream(data []byte) {
	mu.Lock()
	imgBuf = data
	mu.Unlock()
}

func captureStream(ctx context.Context) chromedp.Action {
	return chromedp.ActionFunc(func(c context.Context) error {
		var buf []byte
		if err := chromedp.CaptureScreenshot(&buf).Do(c); err == nil {
			updateStream(buf)
		}
		return nil
	})
}
