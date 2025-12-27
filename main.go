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
	CaptureDir        = "./captures"
)

// --- Global State ---
var (
	// FIX: Variable declaration ab theek hai
	isRunning bool       // Bot chal raha hai ya nahi
	mu        sync.Mutex // Safety lock
)

func main() {
	// 1. Capture Directory banayein
	if _, err := os.Stat(CaptureDir); os.IsNotExist(err) {
		os.Mkdir(CaptureDir, 0777)
	}

	// 2. Setup Web Server
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	
	// Saved images ko browser par dikhane k liye allow karein
	r.Static("/captures", "./captures")

	// Home Page
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// Gallery API (Images ki list dega)
	r.GET("/gallery", func(c *gin.Context) {
		files, _ := filepath.Glob(filepath.Join(CaptureDir, "*.jpg"))
		sort.Strings(files) // Puraani se nayi tartib
		
		var images []string
		for _, f := range files {
			images = append(images, "/captures/"+filepath.Base(f))
		}
		c.JSON(200, images)
	})

	// Start Bot API
	r.POST("/start-bot", func(c *gin.Context) {
		mu.Lock()
		if isRunning {
			mu.Unlock()
			c.JSON(400, gin.H{"status": "Busy", "message": "Bot already running!"})
			return
		}
		isRunning = true
		mu.Unlock()

		// Purani pics delete karein
		oldFiles, _ := filepath.Glob(filepath.Join(CaptureDir, "*.jpg"))
		for _, f := range oldFiles {
			os.Remove(f)
		}

		// Bot ko background main chalayein
		go func() {
			defer func() {
				mu.Lock()
				isRunning = false
				mu.Unlock()
			}()
			runHuaweiBot(TargetPhoneNumber)
		}()

		c.JSON(200, gin.H{"status": "Started", "number": TargetPhoneNumber})
	})

	log.Println("Server running on port", Port)
	r.Run(Port)
}

// --- Bot Logic ---
func runHuaweiBot(phoneNumber string) {
	log.Println("Starting Bot Logic for:", phoneNumber)

	// Browser Setup
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Mobile Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	frameID := 0
	
	// Helper: Save Image to Disk
	saveFrame := func(tag string) chromedp.ActionFunc {
		return func(c context.Context) error {
			var buf []byte
			if err := chromedp.CaptureScreenshot(&buf).Do(c); err != nil {
				return err
			}
			frameID++
			// File name: frame_001_tag.jpg
			filename := fmt.Sprintf("%s/frame_%03d_%s.jpg", CaptureDir, frameID, tag)
			return os.WriteFile(filename, buf, 0644)
		}
	}

	err := chromedp.Run(ctx,
		chromedp.Navigate(TargetURL),
		chromedp.Sleep(5*time.Second),
		saveFrame("1_loaded"),

		// Select Country
		chromedp.Click(`//div[contains(@class, 'hwid-input-div')]`, chromedp.NodeVisible),
		chromedp.Sleep(2*time.Second),
		saveFrame("2_dropdown"),

		chromedp.SendKeys(`input[type="search"]`, "Pakistan"),
		chromedp.Sleep(2*time.Second),
		saveFrame("3_search"),

		chromedp.Click(`//li[contains(text(), 'Pakistan')]`, chromedp.NodeVisible),
		chromedp.Sleep(2*time.Second),
		saveFrame("4_selected"),

		// Input Number
		chromedp.SendKeys(`input[type="tel"]`, phoneNumber),
		chromedp.Sleep(1*time.Second),
		saveFrame("5_number_typed"),

		// Click Get Code
		chromedp.Click(`//div[contains(text(), 'Get code')]`, chromedp.NodeVisible),
		saveFrame("6_clicked_get_code"),

		// Recording Mode (Watch for 30 seconds)
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
		log.Printf("Bot Error: %v", err)
	} else {
		log.Println("Bot Finished Successfully")
	}
}
