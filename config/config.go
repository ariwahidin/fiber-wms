package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

var (
	MAIN_ROUTES   string
	GUEST_ROUTES  string
	APP_PORT      string
	JWTSecret     string
	JWTExpiration int

	DBDriver   string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBUnit     string

	CookieSecure   bool
	CookieHTTPOnly bool
	CookieSameSite string

	allowedOrigins map[string]bool
)

// LoadConfig membaca file .env dan menginisialisasi variabel konfigurasi
func LoadConfig() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Server Configuration
	MAIN_ROUTES = getEnv("MAIN_ROUTES", "/api/v1")
	GUEST_ROUTES = getEnv("GUEST_ROUTES", "/guest/api/v1")
	APP_PORT = getEnv("APP_PORT", "9000")

	// JWT Configuration
	JWTSecret = getEnv("JWT_SECRET", "wms_express_laravel_key_secret")
	JWTExpiration = getEnvAsInt("JWT_EXPIRATION", 86400)

	// Database Configuration
	DBDriver = getEnv("DB_DRIVER", "mssql")
	DBHost = getEnv("DB_HOST", "localhost")
	DBPort = getEnv("DB_PORT", "1433")
	DBUser = getEnv("DB_USER", "golang")
	DBPassword = getEnv("DB_PASSWORD", "P@ssw012d!")
	DBUnit = getEnv("DB_UNIT", "yuwell_uat")

	// Cookie Configuration
	CookieSecure = getEnvAsBool("COOKIE_SECURE", true)
	CookieHTTPOnly = getEnvAsBool("COOKIE_HTTPONLY", false)
	CookieSameSite = getEnv("COOKIE_SAMESITE", "None")

	// Load Allowed Origins
	loadAllowedOrigins()
}

// getEnv membaca environment variable dengan nilai default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt membaca environment variable sebagai integer
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

// getEnvAsBool membaca environment variable sebagai boolean
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}

// loadAllowedOrigins memuat daftar origin yang diizinkan dari environment variable
func loadAllowedOrigins() {
	allowedOrigins = make(map[string]bool)
	originsStr := getEnv("ALLOWED_ORIGINS", "")

	if originsStr == "" {
		// Default origins jika tidak ada di .env
		allowedOrigins = map[string]bool{
			"http://127.0.0.1:3000": true,
		}
		return
	}

	origins := strings.Split(originsStr, ",")
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowedOrigins[origin] = true
		}
	}
}

func SetupCORS(app *fiber.App) {
	app.Use(func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		if allowedOrigins[origin] {
			c.Set("Access-Control-Allow-Origin", origin)
			c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
			c.Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight request
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	})
}

func GetTokenCookie(token string) *fiber.Cookie {
	return &fiber.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: CookieHTTPOnly,
		SameSite: CookieSameSite,
		Path:     "/",
		Secure:   CookieSecure,
	}
}

// package config

// import (
// 	"time"

// 	"github.com/gofiber/fiber/v2"
// )

// const MAIN_ROUTES = "/api/v1"
// const GUEST_ROUTES = "/guest/api/v1"
// const APP_PORT = "9000"
// const JWTSecret = "wms_express_laravel_key_secret" // Ganti dengan secret key yang kuat
// const JWTExpiration = 24 * 60 * 60

// // KONFIGURASI DATABASE MSSQL
// const DBDriver = "mssql"
// const DBHost = "localhost"
// const DBPort = "1433"
// const DBUser = "golang"
// const DBPassword = "P@ssw012d!"
// const DBUnit = "yuwell_uat"

// // KONFIGURASI DATABASE POSTGRESQL
// // const DBDriver = "postgres" // Bisa: "postgres", "mysql", "mssql"
// // const DBHost = "localhost"
// // const DBPort = "5433"         // Default port PostgreSQL
// // const DBUser = "postgres"     // Sesuaikan dengan user PostgreSQL kamu
// // const DBPassword = "password" // Sesuaikan dengan password user PostgreSQL kamu
// // const DBName = "backend_wms"  // Pastikan sudah dibuat di PostgreSQL

// var allowedOrigins = map[string]bool{
// 	"http://127.0.0.1:3000":            true,
// 	"http://172.19.169.106:3000":       true,
// 	"http://172.19.170.105:3000":       true,
// 	"http://172.19.170.169:3000":       true,
// 	"http://192.168.168.22:3000":       true,
// 	"http://192.168.168.22:4800":       true,
// 	"http://103.111.191.152:8011":      true,
// 	"http://103.111.191.152:8012":      true,
// 	"http://192.168.40.1:3000":         true,
// 	"http://192.168.40.1:8083":         true,
// 	"http://127.0.0.1:8083":            true,
// 	"https://laracv.com":               true,
// 	"https://next-wms-zeta.vercel.app": true,
// }

// func SetupCORS(app *fiber.App) {
// 	app.Use(func(c *fiber.Ctx) error {
// 		origin := c.Get("Origin")
// 		if allowedOrigins[origin] {
// 			c.Set("Access-Control-Allow-Origin", origin)
// 			c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
// 			c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
// 			c.Set("Access-Control-Allow-Credentials", "true")
// 		}

// 		// Handle preflight request
// 		if c.Method() == fiber.MethodOptions {
// 			return c.SendStatus(fiber.StatusNoContent)
// 		}
// 		return c.Next()
// 	})
// }

// func GetTokenCookie(token string) *fiber.Cookie {
// 	return &fiber.Cookie{
// 		// Name:     "token",
// 		// Value:    token,
// 		// Expires:  time.Now().Add(24 * time.Hour),
// 		// HTTPOnly: true,
// 		// SameSite: "None",
// 		// Secure:   true,
// 		Name:     "refresh_token",
// 		Value:    token,
// 		Expires:  time.Now().Add(24 * time.Hour),
// 		HTTPOnly: false,  // ✅ Sekarang bisa dibaca oleh middleware
// 		SameSite: "None", // ✅ Lax atau Strict disarankan untuk keamanan
// 		Path:     "/",    // ✅ Pastikan bisa diakses semua path
// 		Secure:   true,   // ✅ Tetap aman di HTTPS (Vercel)
// 	}
// }
