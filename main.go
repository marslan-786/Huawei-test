package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
)

// --- Configuration ---
const (
	TargetPhoneNumber = "3177635849" // Apna number yahan likhein
	TargetURL         = "https://id5.cloud.huawei.com/CAS/mobile/standard/register/wapRegister.html?reqClientType=7&loginChannel=7000000&regionCode=hk&loginUrl=https%3A%2F%2Fid5.cloud.huawei.com%2FCAS%2Fmobile%2Fstandard%2FwapLogin.html&lang=en-us&themeName=huawei#/wapRegister/regByPhone"
	Port              = ":8080"
	CaptureDir        = "./captures" // Tasveeren yahan save hongi
)

var (
	mu isRunning bool // Bot running status check karne k liye
)

func main() {
	// 1. Ensure Capture Directory Exists
	if _, err := os.Stat(CaptureDir); os.IsNotExist(err) {
		os.Mkdir(CaptureDir, 0755)
	}

	// 2. Start Server
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	
	// Static files serve karein (Taake saved images browser main khul sakein)
	r.Static("/captures", "./captures")

	// Home Page
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// API: Get List of Saved Images (Refresh karne k liye)
	r.GET("/gallery", func(c *gin.Context) {
		files, _ := filepath.Glob(filepath.Join(CaptureDir, "*.jpg"))
		// Sort files to ensure order (frame_001, frame_002...)
		sort.Strings(files)
		
		// Paths clean karein web k liye
		var images []string
		for _, f := range files {
			images = append(images, "/captures/"+filepath.Base(f))
		}
		c.JSON(200, images)
	})

	// API: Start Bot
	r.POST("/start-bot", func(c *gin.Context) {
		if mu {
			c.JSON(400, gin.H{"status": "Already Running", "number": TargetPhoneNumber})
			return
		}
		// Purani tasveeren delete karein (Fresh Start)
		oldFiles, _ := filepath.Glob(filepath.Join(CaptureDir, "*.jpg"))
		for _, f := range oldFiles {
			os.Remove(f)
		}

		go runHuaweiBot(TargetPhoneNumber)
		c.JSON(200, gin.H{"status": "Started", "number": TargetPhoneNumber})
	})

	log.Println("Server running on port", Port)
	r.Run(Port)
}

// --- Bot Logic ---
func runHuaweiBot(phoneNumber string) {
	mu = true
	defer func() { mu = false }()

	log.Println("Testing Number:", phoneNumber)

	// Browser Setup (Headless but stable)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true), // Memory crash se bachata hai
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Timeout set karein taake agar atak jaye to 2 minute baad band ho
	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Frame Counter
	frameID := 0
	
	// Helper to save screenshot to DISK
	saveFrame := func(stepName string) chromedp.ActionFunc {
		return func(c context.Context) error {
			var buf []byte
			if err := chromedp.CaptureScreenshot(&buf).Do(c); err != nil {
				return err
			}
			frameID++
			filename := fmt.Sprintf("%s/frame_%03d_%s.jpg", CaptureDir, frameID, stepName)
			return os.WriteFile(filename, buf, 0644)
		}
	}

	err := chromedp.Run(ctx,
		// 1. Navigate
		chromedp.Navigate(TargetURL),
		chromedp.Sleep(2*time.Second),
		saveFrame("loaded"),

		// 2. Select Country (Pakistan +92)
		// Check karein k dropdown visible hai
		chromedp.WaitVisible(`//div[contains(@class, 'hwid-input-div')]`),
		chromedp.Click(`//div[contains(@class, 'hwid-input-div')]`),
		chromedp.Sleep(1*time.Second),
		saveFrame("dropdown_clicked"),

		// Search Pakistan
		chromedp.SendKeys(`input[type="search"]`, "Pakistan"),
		chromedp.Sleep(1*time.Second),
		saveFrame("typed_pakistan"),

		// Click Result
		chromedp.Click(`//li[contains(text(), 'Pakistan')]`),
		chromedp.Sleep(1*time.Second),
		saveFrame("country_selected"),

		// 3. Enter Number
		chromedp.WaitVisible(`input[type="tel"]`),
		chromedp.SendKeys(`input[type="tel"]`, phoneNumber),
		chromedp.Sleep(1*time.Second),
		saveFrame("number_entered"),

		// 4. Click Get Code
		chromedp.Click(`//div[contains(text(), 'Get code')]`),
		saveFrame("clicked_get_code"),

		// 5. Recording Phase (Video Loop)
		// Agle 30 second tak har second tasveer save karega
		chromedp.ActionFunc(func(c context.Context) error {
			for i := 0; i < 30; i++ {
				var buf []byte
				chromedp.CaptureScreenshot(&buf).Do(c)
				frameID++
				filename := fmt.Sprintf("%s/frame_%03d_monitor.jpg", CaptureDir, frameID)
				os.WriteFile(filename, buf, 0644)
				
				time.Sleep(1 * time.Second)
			}
			return nil
		}),
	)

	if err != nil {
		log.Printf("Bot Crashed/Stopped: %v", err)
		// Error ka screenshot bhi lein
		// (Alag context banana padega agar purana mar gaya, lekin log kafi hai)
	}
	log.Println("Bot Finished Successfully")
}
