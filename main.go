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
	// Setup Folders
	os.RemoveAll(CaptureDir) // Purana data saf karein start par
	os.Mkdir(CaptureDir, 0777)

	// Start Bot Loop
	go startInfiniteBot()

	// Web Server
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

	r.GET("/make-video", func(c *gin.Context) {
		outputFile := filepath.Join(CaptureDir, "output.mp4")
		os.Remove(outputFile)
		// FFmpeg command updated for reliability
		cmd := exec.Command("bash", "-c", fmt.Sprintf("ffmpeg -y -framerate 1 -pattern_type glob -i '%s/*.jpg' -c:v libx264 -pix_fmt yuv420p %s", CaptureDir, outputFile))
		output, err := cmd.CombinedOutput()
		if err != nil {
			c.JSON(500, gin.H{"error": "Video failed", "details": string(output)})
			return
		}
		c.JSON(200, gin.H{"video_url": "/captures/output.mp4"})
	})

	r.Run(Port)
}

func startInfiniteBot() {
	// Infinite loop lekin ab ye bar bar reload nahi karega jab tak cycle complete na ho
	for {
		log.Println(">>> Starting Full Cycle...")
		runBotSequence()
		log.Println(">>> Cycle Complete. Waiting 30 seconds before restart...")
		time.Sleep(30 * time.Second) // Cycle k baad lamba wait
	}
}

func runBotSequence() {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	
	// Timeout barha diya taake beech main band na ho
	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	ts := time.Now().Unix()
	
	// --- Capture Helper ---
	capture := func(name string) chromedp.ActionFunc {
		return func(c context.Context) error {
			var buf []byte
			// Full page screenshot ki bajaye viewport lein (speed k liye)
			chromedp.CaptureScreenshot(&buf).Do(c)
			filename := fmt.Sprintf("%s/%d_%s.jpg", CaptureDir, ts, name)
			return os.WriteFile(filename, buf, 0644)
		}
	}

	// --- Visual Click Helper (Mouse ka nishan lagata hai) ---
	visualizeAndClick := func(xpath string, stepName string) chromedp.ActionFunc {
		return func(c context.Context) error {
			// 1. Pehle JS inject karein jo us element par ungli (👆) aur border banaye
			js := fmt.Sprintf(`
				var el = document.evaluate("%s", document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
				if(el) {
					el.style.border = "5px solid red";
					el.style.backgroundColor = "rgba(255, 255, 0, 0.3)";
					
					var pointer = document.createElement("div");
					pointer.innerHTML = "👆";
					pointer.style.fontSize = "50px";
					pointer.style.position = "absolute";
					pointer.style.zIndex = "99999";
					var rect = el.getBoundingClientRect();
					pointer.style.left = (rect.left + window.scrollX + 20) + "px";
					pointer.style.top = (rect.top + window.scrollY + 20) + "px";
					document.body.appendChild(pointer);
				}
			`, xpath)
			
			chromedp.Evaluate(js, nil).Do(c)
			
			// 2. Ab Screenshot lein taake humein click hota nazar aye
			capture(stepName + "_aiming")(c)
			time.Sleep(1 * time.Second) // Thora wait taake screenshot main aa jaye

			// 3. Ab asal click karein
			return chromedp.Click(xpath, chromedp.NodeVisible).Do(c)
		}
	}

	log.Println("Navigating...")
	
	err := chromedp.Run(ctx,
		// 1. Load Page
		chromedp.Navigate(TargetURL),
		chromedp.Sleep(5*time.Second), // Load hone ka sakoon se wait
		capture("01_page_loaded"),

		// 2. Click Country Dropdown (With Visual Indicator)
		visualizeAndClick(`//div[contains(text(), 'Region') or contains(text(), 'Country')]`, "02_click_country"),
		chromedp.Sleep(2*time.Second),
		capture("03_country_list_open"),

		// 3. Type Pakistan (Search)
		chromedp.SendKeys(`input[type="search"]`, "Pakistan"),
		chromedp.Sleep(2*time.Second),
		capture("04_typed_pakistan"),

		// 4. Click Pakistan Option (With Visual Indicator)
		visualizeAndClick(`//li[contains(text(), 'Pakistan')]`, "05_select_pakistan"),
		chromedp.Sleep(2*time.Second),
		capture("06_pakistan_selected"),

		// 5. Input Number
		chromedp.SendKeys(`input[type="tel"]`, TargetPhoneNumber),
		chromedp.Sleep(1*time.Second),
		capture("07_number_entered"),

		// 6. Click Get Code (With Visual Indicator)
		visualizeAndClick(`//div[contains(text(), 'Get code')]`, "08_click_get_code"),
		
		// 7. Monitoring Phase (Ab ye page refresh nahi karega, bas dekhta rahega)
		chromedp.ActionFunc(func(c context.Context) error {
			log.Println("Monitoring Screen for results...")
			for i := 1; i <= 20; i++ { // 20 screenshots lay ga (approx 40 seconds)
				capture(fmt.Sprintf("09_monitor_%02d", i))(c)
				time.Sleep(2 * time.Second)
			}
			return nil
		}),
	)

	if err != nil {
		log.Printf("Bot Error: %v", err)
		// Error ka screenshot
		var buf []byte
		chromedp.CaptureScreenshot(&buf).Do(ctx)
		os.WriteFile(fmt.Sprintf("%s/%d_99_ERROR_SCREEN.jpg", CaptureDir, ts), buf, 0644)
	}
}