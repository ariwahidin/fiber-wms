package middleware

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// warna ANSI
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

// handlerName ambil nama fungsi dari reflect runtime
func handlerName(h fiber.Handler) string {
	full := runtime.FuncForPC(
		reflect.ValueOf(h).Pointer(),
	).Name()
	// ambil bagian terakhir: "controllers.GetProducts-fm" → "GetProducts"
	parts := strings.Split(full, ".")
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, "-fm")
	return name
}

func ConsoleLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		lat := time.Since(start)
		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()

		// cari handler yang match route ini
		fnName := "?"
		for _, routes := range c.App().Stack() {
			for _, route := range routes {
				if route.Method == method && route.Path == c.Route().Path {
					if len(route.Handlers) > 0 {
						fnName = handlerName(
							route.Handlers[len(route.Handlers)-1],
						)
					}
					break
				}
			}
		}

		// format latency: hijau <50ms, kuning <500ms, merah >=500ms
		latStr := fmt.Sprintf("%v", lat.Round(time.Millisecond))
		var latColor string
		switch {
		case lat < 50*time.Millisecond:
			latColor = green
		case lat < 500*time.Millisecond:
			latColor = yellow
		default:
			latColor = red
		}

		fmt.Printf(
			"%s%s%s %-7s%s %s%-20s%s %s%d%s %s%s%s  %s→ %s%s\n",
			gray, time.Now().Format("15:04:05"), reset,
			methodColor(method)+bold+method+reset, reset,
			cyan, path, reset,
			statusColor(status)+bold, status, reset,
			latColor, latStr, reset,
			gray, fnName, reset,
		)

		return err
	}
}
