package logger

// Example usage of the colored logger
//
// To use this logger in your code, simply replace:
//   log.Printf("[Component] Message")
// with:
//   logger.Printf("[Component] Message")
//
// Example migration:
//
// Before:
//   import "log"
//   log.Printf("[RabbitMQ] Connected to queue: %s", queueName)
//   log.Printf("[Schedule] Failed to find patient: %v", err)
//
// After:
//   import "github.com/aseno/backend/internal/logger"
//   logger.Printf("[RabbitMQ] Connected to queue: %s", queueName)
//   logger.Printf("[Schedule] Failed to find patient: %v", err)
//
// Available functions:
//   - logger.Printf(format, args...)   - Formatted print with colored brackets
//   - logger.Println(args...)          - Print line with colored brackets
//   - logger.Print(args...)            - Print with colored brackets
//   - logger.Fatalf(format, args...)   - Fatal error (exits program)
//   - logger.Panicf(format, args...)   - Panic
//
// Additional helper functions:
//   - logger.Errorf(format, args...)   - Adds [ERROR] prefix
//   - logger.Warnf(format, args...)    - Adds [WARN] prefix
//   - logger.Infof(format, args...)    - Adds [INFO] prefix
//   - logger.Successf(format, args...) - Adds [SUCCESS] prefix
//   - logger.Debugf(format, args...)   - Adds [DEBUG] prefix
//
// Custom colors:
// You can register custom colors for specific bracket contents:
//   import "github.com/fatih/color"
//   logger.RegisterColor("MyComponent", color.New(color.FgRed, color.Bold))
//
// Disable colors:
//   logger.DisableColor()  // Useful for testing or file output
//
// Initialization (optional, but recommended in main.go):
//   func main() {
//       logger.Init()  // Sets up default configuration
//       // ... rest of your code
//   }
