package main

import (
	"fmt"
	"os"
	"time"

	"github.com/AYColumbia/viberowser/ui"
)

func main() {
	fmt.Println("Viberowser - A Web Browser")

	// Check if we should run in headless mode for testing
	if len(os.Args) > 1 && os.Args[1] == "--headless" {
		fmt.Println("Running in headless mode...")
		return
	}

	// Create and run the browser UI
	browser := ui.NewBrowserUI()

	// If a URL was provided as argument, navigate to it after a short delay
	if len(os.Args) > 1 {
		url := os.Args[1]
		go func() {
			// Give the UI time to initialize
			time.Sleep(100 * time.Millisecond)
			browser.NavigateToURL(url)
		}()
	}

	browser.Run()
}
