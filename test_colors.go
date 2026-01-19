package main

import (
	"github.com/aseno/backend/internal/logger"
	"github.com/fatih/color"
)

func main() {
	// Force colors enabled
	color.NoColor = false

	logger.Init()
	logger.EnableColor()

	// Test direct color library
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.Println("Direct fatih/color test - should be CYAN and BOLD")

	// Test our logger
	logger.Printf("[RabbitMQ] Connected to queue: notifications")
	logger.Printf("[Schedule] Processing appointment")
	logger.Printf("[ERROR] This is an error")
	logger.Errorf("This is an error with Errorf")
}
