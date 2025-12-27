package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
)

// --- Configuration ---
const (
	TargetPhoneNumber = "3177635849"
	TargetURL         = "https://id5.cloud.huawei.com/CAS/mobile/standard/register/wapRegister.html?reqClientType=7&loginChannel=7000000&regionCode=hk&loginUrl=https%3A%2F%2Fid5.cloud.huawei.com%2FCAS%2Fmobile%2Fstandard%2FwapLogin.html&lang=en-us&themeName=huawei#/wapRegister/regByPhone"
	Port              = ":8080"
	CaptureDir        = "./captures"
)

func main() {
	// 1. Setup Directories
	os.Mkdir(CaptureDir, 0777)

	// 2. Start Bot in Background (FOREVER LOOP)
	go startInfiniteBot()

	// 3. Setup Web Server
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/captures", CaptureDir)

	// Dashboard
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// Gallery API
	r.GET("/gallery-data", func(c *gin.Context) {
		files, _ := filepath.Glob(filepath.Join(CaptureDir, "frame_*.jpg"))
		sort.Strings(files)
		var images []string
		for _, f := range files {
			images = append(images, "/captures/"+filepath.Base(f))
		}
		c.JSON(200, images)
	})

	// Make Video API (FFmpeg)
	r.GET("/make-video", func(c *gin.Context) {
		outputFile := filepath.Join(CaptureDir, "output.mp4")
		// Delete old video
		os.Remove(outputFile)

		// Run FFmpeg: Convert jpg sequence to mp4
		// -framerate 2: matlab 1 second main 2 frames (tez video)
		cmd := exec.Command("ffmpeg", "-y", "-framerate", "2", "-pattern_type", "glob", "-i", CaptureDir+"/frame_*.jpg", "-c:v", "libx264", "-pix_fmt", "yuv420p", outputFile)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Println("FFmpeg Error:", string(output))
			c.JSON(500, gin.H{"error": "Video generation failed", "details": string(output)})
			return
		}

		c.JSON(200, gin.H{"video_url": "/captures/output.mp4"})
	})

	log.Println("System Online. Bot starting automatically...")
	r.Run(Port)
}

// --- The Infinite Loop ---
func startInfiniteBot() {
	for {
		log.Println(">>> Starting New Cycle...")
		
		// Clean old files before new run (Optional: agar purana data clear karna ho)
		// os.RemoveAll(CaptureDir) 
		// os.Mkdir(CaptureDir, 0777)

		runBotSequence()
		
		log.Println(">>> Cycle Finished. Restarting in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

// --- Bot Logic ---
func runBotSequence() {
	// Browser Config
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

	// Context with Timeout (Taake agar atak jaye to khud band ho jaye)
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	frameID := 0
	timestamp := time.Now().Unix() // Unique batch ID for filenames

	// Helper to capture
	capture := func(tag string) chromedp.ActionFunc {
		return func(c context.Context) error {
			var buf []byte
			if err := chromedp.CaptureScreenshot(&buf).Do(c); err != nil {
				return err
			}
			frameID++
			// File Name: frame_TIMESTAMP_001_tag.jpg (Sorting k liye best)
			filename := fmt.Sprintf("%s/frame_%d_%03d_%s.jpg", CaptureDir, timestamp, frameID, tag)
			return os.WriteFile(filename, buf, 0644)
		}
	}

	// Automation Steps
	err := chromedp.Run(ctx,
		chromedp.Navigate(TargetURL),
		capture("1_start"), // Pehla screenshot
		
		chromedp.Sleep(5*time.Second),
		capture("2_loaded"),

		// Try Click Dropdown (Using generic selector to avoid crash)
		// Hum 'hwid-input-div' class dhoond rahe hain jo dropdown hai
		chromedp.WaitVisible(`//div[contains(@class, 'hwid-input-div')]`),
		chromedp.Click(`//div[contains(@class, 'hwid-input-div')]`),
		chromedp.Sleep(2*time.Second),
		capture("3_dropdown_click"),

		// Type Pakistan
		chromedp.SendKeys(`input[type="search"]`, "Pakistan"),
		chromedp.Sleep(2*time.Second),
		capture("4_typed_pakistan"),

		// Click Pakistan List Item
		chromedp.Click(`//li[contains(text(), 'Pakistan')]`),
		chromedp.Sleep(2*time.Second),
		capture("5_selected_pakistan"),

		// Enter Number
		chromedp.SendKeys(`input[type="tel"]`, TargetPhoneNumber),
		chromedp.Sleep(1*time.Second),
		capture("6_entered_number"),

		// Click Get Code
		chromedp.Click(`//div[contains(text(), 'Get code')]`),
		capture("7_clicked_get_code"),

		// Monitoring Loop (30 Seconds)
		chromedp.ActionFunc(func(c context.Context) error {
			for i := 0; i < 15; i++ { // 15 screenshots lenge
				capture(fmt.Sprintf("monitor_%d", i))(c)
				time.Sleep(1 * time.Second)
			}
			return nil
		}),
	)

	if err != nil {
		log.Printf("Bot Error (Screenshot taken): %v", err)
	}
}
