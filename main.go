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

	"github.com/chromedp/cdproto/input"
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
	os.Mkdir(CaptureDir, 0777)
	go startInfiniteBot()

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/captures", CaptureDir)

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.GET("/gallery-data", func(c *gin.Context) {
		files, _ := filepath.Glob(filepath.Join(CaptureDir, "*.jpg"))
		sort.Strings(files)
		var images []string
		for _, f := range files {
			images = append(images, "/captures/"+filepath.Base(f))
		}
		c.JSON(200, images)
	})

	// FIXED VIDEO COMMAND (Using mpeg4 instead of x264)
	r.GET("/make-video", func(c *gin.Context) {
		outputFile := filepath.Join(CaptureDir, "output.mp4")
		os.Remove(outputFile)

		// Command changed to use 'mpeg4' codec which is always available
		cmdStr := fmt.Sprintf("ffmpeg -y -framerate 1 -pattern_type glob -i '%s/*.jpg' -c:v mpeg4 -q:v 5 %s", CaptureDir, outputFile)
		cmd := exec.Command("bash", "-c", cmdStr)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Println("FFmpeg Error:", string(output))
			c.JSON(500, gin.H{"error": "Video failed", "details": string(output)})
			return
		}
		c.JSON(200, gin.H{"video_url": "/captures/output.mp4"})
	})

	r.Run(Port)
}

func startInfiniteBot() {
	for {
		// Clean old files every cycle to save space
		files, _ := filepath.Glob(filepath.Join(CaptureDir, "*.jpg"))
		if len(files) > 100 { // Keep manageable
			os.RemoveAll(CaptureDir)
			os.Mkdir(CaptureDir, 0777)
		}

		log.Println(">>> Starting New Cycle...")
		runBotSequence()
		log.Println(">>> Cycle Finished. Restarting in 10 seconds...")
		time.Sleep(10 * time.Second)
	}
}

func runBotSequence() {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1080, 1920), // Mobile Resolution
		chromedp.UserAgent("Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Mobile Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()

	ts := time.Now().Unix()
	
	capture := func(name string) chromedp.ActionFunc {
		return func(c context.Context) error {
			var buf []byte
			chromedp.CaptureScreenshot(&buf).Do(c)
			filename := fmt.Sprintf("%s/%d_%s.jpg", CaptureDir, ts, name)
			return os.WriteFile(filename, buf, 0644)
		}
	}

	// --- Enhanced Visual Click ---
	smartClick := func(xpath string, stepName string) chromedp.ActionFunc {
		return func(c context.Context) error {
			// 1. Draw Red Dot
			js := fmt.Sprintf(`
				var el = document.evaluate("%s", document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
				if(el) {
					var dot = document.createElement("div");
					dot.style.width = "20px";
					dot.style.height = "20px";
					dot.style.background = "red";
					dot.style.borderRadius = "50%%";
					dot.style.position = "absolute";
					dot.style.zIndex = "10000";
					dot.style.border = "2px solid white";
					var rect = el.getBoundingClientRect();
					dot.style.left = (rect.left + (rect.width/2)) + "px";
					dot.style.top = (rect.top + (rect.height/2)) + "px";
					document.body.appendChild(dot);
				}
			`, xpath)
			chromedp.Evaluate(js, nil).Do(c)
			
			capture(stepName + "_aiming")(c)
			time.Sleep(500 * time.Millisecond)

			// 2. Try Standard Click
			err := chromedp.Click(xpath, chromedp.NodeVisible).Do(c)
			if err != nil {
				log.Printf("Standard click failed for %s, trying JS Click...", stepName)
				// 3. Fallback: JavaScript Click (Works even if overlapped)
				jsClick := fmt.Sprintf(`
					var el = document.evaluate("%s", document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
					if(el) el.click();
				`, xpath)
				return chromedp.Evaluate(jsClick, nil).Do(c)
			}
			return nil
		}
	}

	log.Println("Navigating...")
	
	err := chromedp.Run(ctx,
		chromedp.Navigate(TargetURL),
		chromedp.Sleep(8*time.Second), // Load k liye lamba wait
		capture("01_loaded"),

		// STEP 0: Close Cookie Banner (Agar aya ho to)
		chromedp.ActionFunc(func(c context.Context) error {
			// Try finding generic "Accept" or "X" buttons
			chromedp.Click(`//span[contains(text(), 'Accept')]`, chromedp.NodeVisible).Do(c)
			return nil
		}),

		// STEP 1: Click "Country/Region" (Using Text Match)
		// Ye sab se safe selector hai
		smartClick(`//*[contains(text(), "Region")]`, "02_click_region"),
		chromedp.Sleep(3*time.Second),
		capture("03_region_list"),

		// STEP 2: Type Pakistan
		chromedp.SendKeys(`input[type="search"]`, "Pakistan"),
		chromedp.Sleep(2*time.Second),
		capture("04_typed_pak"),

		// STEP 3: Click "Pakistan +92"
		smartClick(`//div[contains(text(), "Pakistan")]`, "05_select_pak"),
		chromedp.Sleep(2*time.Second),
		capture("06_pak_selected"),

		// STEP 4: Input Number (Focus + Type)
		// Kabhi kabhi input field par click karna parta hai pehle
		smartClick(`//input[@type="tel"]`, "07_focus_input"),
		chromedp.SendKeys(`//input[@type="tel"]`, TargetPhoneNumber),
		chromedp.Sleep(1*time.Second),
		capture("08_number_entered"),

		// STEP 5: Click Get Code
		smartClick(`//*[contains(text(), "Get code")]`, "09_click_getcode"),
		
		// STEP 6: Watch Loop (No refresh)
		chromedp.ActionFunc(func(c context.Context) error {
			for i := 1; i <= 15; i++ {
				capture(fmt.Sprintf("10_waiting_%02d", i))(c)
				
				// Agar Slider aye to usay handle karne ki logic yahan ayegi baad main
				// Filhal hum sirf dekh rahe hain
				
				time.Sleep(2 * time.Second)
			}
			return nil
		}),
	)

	if err != nil {
		log.Printf("Bot Error: %v", err)
		// Touch Simulation (Last Resort - Center Click)
		// Agar sab fail ho jaye to screen k beech main click karega (Debugging)
		chromedp.Run(ctx, 
			chromedp.ActionFunc(func(c context.Context) error {
				p := &input.DispatchMouseEventParams{
					Type: input.MousePressed,
					X:    500,
					Y:    800,
					Button: input.MouseButtonLeft,
					ClickCount: 1,
				}
				p.Do(c)
				p.Type = input.MouseReleased
				p.Do(c)
				return nil
			}),
			capture("99_panic_click"),
		)
	}
}