package middleware

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	gray   = "\033[90m"
	green  = "\033[32m"
	cyan   = "\033[36m"
	yellow = "\033[33m"
	red    = "\033[31m"
	blue   = "\033[34m"
	purple = "\033[35m"
)

func methodColor(m string) string {
	switch m {
	case "GET":
		return green
	case "POST":
		return blue
	case "PUT":
		return yellow
	case "DELETE":
		return red
	case "PATCH":
		return purple
	default:
		return cyan
	}
}

func statusColor(s int) string {
	switch {
	case s >= 500:
		return red
	case s >= 400:
		return yellow
	case s >= 300:
		return cyan
	default:
		return green
	}
}

func latencyColor(d time.Duration) string {
	switch {
	case d < 50*time.Millisecond:
		return green
	case d < 500*time.Millisecond:
		return yellow
	default:
		return red
	}
}

// Get handler name and source location.
func handlerInfo(h fiber.Handler) (string, string, int) {
	if h == nil {
		return "?", "?", 0
	}

	pc := reflect.ValueOf(h).Pointer()
	fn := runtime.FuncForPC(pc)

	if fn == nil {
		return "?", "?", 0
	}

	fullName := fn.Name()
	parts := strings.Split(fullName, ".")
	name := strings.TrimSuffix(parts[len(parts)-1], "-fm")

	file, line := fn.FileLine(pc)

	return name, file, line
}

func getStringLocal(c *fiber.Ctx, key string) string {
	value := c.Locals(key)

	if value == nil {
		return "-"
	}

	return fmt.Sprint(value)
}

func ConsoleLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {

		start := time.Now()

		err := c.Next()

		latency := time.Since(start)
		status := c.Response().StatusCode()

		method := c.Method()
		path := c.Path()

		// User information from authentication middleware.
		username := getStringLocal(c, "username")
		userID := getStringLocal(c, "userID")

		// Client information.
		ip := c.IP()

		// Find matched handler.
		handler := "?"
		sourceFile := "?"
		sourceLine := 0

		if route := c.Route(); route != nil {
			if len(route.Handlers) > 0 {
				handler, sourceFile, sourceLine =
					handlerInfo(route.Handlers[len(route.Handlers)-1])
			}
		}

		latencyText := latency.Round(time.Millisecond).String()

		fmt.Printf("\n")

		// Request header
		fmt.Printf(
			"%s%s%s ",
			gray,
			time.Now().Format("2006-01-02 15:04:05"),
			reset,
		)

		fmt.Printf(
			"%s%s%s ",
			methodColor(method),
			bold+method,
			reset,
		)

		fmt.Printf(
			"%s%-35s%s ",
			cyan,
			path,
			reset,
		)

		fmt.Printf(
			"%s%s%d%s ",
			statusColor(status),
			bold,
			status,
			reset,
		)

		fmt.Printf(
			"%s%s%s ",
			latencyColor(latency),
			latencyText,
			reset,
		)

		fmt.Printf(
			"%s→ %s%s\n",
			gray,
			handler,
			reset,
		)

		// User information
		fmt.Printf("%sUSER     : %s%s\n", gray, username, reset)
		fmt.Printf("%sUSER ID  : %s%s\n", gray, userID, reset)
		fmt.Printf("%sIP       : %s%s\n", gray, ip, reset)

		// Execution information
		fmt.Printf("%sHANDLER  : %s%s\n", gray, handler, reset)
		fmt.Printf("%sSOURCE   : %s:%d%s\n", gray, sourceFile, sourceLine, reset)
		fmt.Printf("%sLATENCY  : %s%s\n", latencyColor(latency), latencyText, reset)

		fmt.Printf(
			"%s────────────────────────────────────────────────────────────────────%s\n",
			gray,
			reset,
		)

		return err
	}
}
