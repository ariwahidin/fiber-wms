package middleware

import (
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

var secretKey = []byte(os.Getenv("JWT_SECRET")) // Ambil dari .env

func AuthMiddleware(c *fiber.Ctx) error {
	// Ambil token dari header Authorization

	// Ambil token dari cookie
	tokenStringCookie := c.Cookies("token")
	// fmt.Println("tokenString: ", tokenStringCookie)

	if tokenStringCookie == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Unauthorized: Missing token",
		})
	}

	// fmt.Println("tokenStringCookie: ", tokenStringCookie)

	// Parse dan validasi token
	token, err := jwt.Parse(tokenStringCookie, func(token *jwt.Token) (interface{}, error) {
		// Pastikan metode signing yang digunakan sesuai
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: Invalid signing method")
		}
		// Kembalikan secret key untuk verifikasi
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	// Handle error saat parsing token
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Unauthorized: Invalid tokenx",
			"error":   err.Error(),
		})
	}

	// Cek apakah token valid
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {

		// fmt.Println("Data claims: ", claims)

		// Cek waktu kedaluwarsa token
		exp, ok := claims["exp"].(float64)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: Invalid expiration time",
			})
		}
		expTime := int64(exp)

		// fmt.Println("expTime: ", expTime)

		currentTime := time.Now().Unix()

		// fmt.Println("currentTime: ", currentTime)
		remainingTime := expTime - currentTime

		// fmt.Println("remainingTime: ", remainingTime)
		// fmt.Printf("userID: %v\n", claims["userID"])

		// Jika sisa waktu kurang dari 5 menit, buat token baru
		if remainingTime < 5*60 {
			userID, ok := claims["userID"].(float64) // Sesuaikan tipe data dengan yang digunakan di token
			if !ok {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"message": "Unauthorized: Invalid user ID",
				})
			}

			newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"userID": userID,
				"exp":    time.Now().Add(60 * time.Minute).Unix(), // Perpanjang waktu ke 60 menit
			})

			newTokenString, err := newToken.SignedString(secretKey)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"message": "Failed to generate new token",
				})
			}

			c.Set("X-New-Token", newTokenString) // Kirim token baru di header response
		}

		// Simpan data user ke context
		userID, ok := claims["userID"].(float64) // Sesuaikan tipe data dengan yang digunakan di token
		// fmt.Println("userID: ", userID)

		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: Invalid user ID s",
			})
		}

		c.Locals("userID", userID)
		c.Locals("userData", claims)
		return c.Next() // Lanjut ke handler berikutnya
	} else {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Unauthorized: Invalid token",
		})
	}
}
