package middleware

import (
	"fiber-app/config"
	"fiber-app/controllers/configurations"
	"fiber-app/models"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type AuthMiddlewareStruct struct {
	DB *gorm.DB
}

// func NewAuthMiddleware(DB *gorm.DB) *AuthMiddlewareStruct {
// 	return &AuthMiddlewareStruct{DB: DB}
// }

// var secretKey = []byte(config.JWTSecret) // Ambil dari .env

func AuthMiddleware(ctx *fiber.Ctx) error {
	// Ambil header Authorization
	authHeader := ctx.Get("Authorization")
	if authHeader == "" {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Missing Authorization header",
		})
	}

	// Ambil token dari "Bearer <token>"
	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || strings.ToLower(tokenParts[0]) != "bearer" {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Invalid Authorization header format",
		})
	}
	tokenStringHeader := tokenParts[1]

	fmt.Println("tokenStringHeader: ", tokenStringHeader)

	// Ambil token dari cookie
	// tokenStringCookie := ctx.Cookies("x_token")
	// fmt.Println("tokenString: ", tokenStringCookie)

	// if tokenStringCookie == "" {
	// 	return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
	// 		"message": "Unauthorized: Token not found in cookie",
	// 	})
	// }

	// fmt.Println("tokenStringCookie: ", tokenStringCookie)

	// Parse dan validasi token
	token, err := jwt.Parse(tokenStringHeader, func(token *jwt.Token) (interface{}, error) {
		// Pastikan metode signing yang digunakan sesuai
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: Invalid signing method")
		}
		// Kembalikan secret key untuk verifikasi
		return []byte(config.JWTSecret), nil
	})

	// fmt.Println("token: ", token)
	// check sisa waktu token dalam string
	// fmt.Println("token.Valid: ", token.Valid)

	fmt.Println("Data token: ", token.Claims)

	// Handle error saat parsing token
	if err != nil {
		fmt.Println("Error parsing token: ", err)
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Unauthorized: Invalid token",
			"error":   err.Error(),
		})
	}

	// Cek apakah token valid
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {

		fmt.Println("Token valid")
		fmt.Println("Data claims: ", claims)

		// Cek waktu kedaluwarsa token
		exp, ok := claims["exp"].(float64)
		if !ok {
			fmt.Println("Token telah kedaluwarsa")
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: Invalid expiration time",
			})
		}
		expTime := int64(exp)

		userID, ok := claims["userID"].(float64)
		if !ok {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: Invalid user ID",
			})
		}

		unit, ok := claims["unit"].(string)

		if !ok {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: Invalid unit",
			})
		}

		fmt.Println("Token masih valid")
		fmt.Println("Waktu kedaluwarsa:", time.Unix(expTime, 0))
		fmt.Println("Sisa waktu:", time.Until(time.Unix(expTime, 0)))
		fmt.Println("UserID: ", userID)

		// Simpan userID dan unit ke context
		ctx.Locals("userID", userID)
		ctx.Locals("unit", unit)
		ctx.Locals("userData", claims)

		// 🔑 Panggil GetDBConnection di sini
		_, err := configurations.GetDBConnection(unit)
		if err != nil {
			return ctx.Status(500).JSON(fiber.Map{"message": "Failed to connect database"})
		} else {
			fmt.Println("Connected to database:", unit)
		}
		configurations.PrintActiveDBConnections()

		return ctx.Next() // Lanjut ke handler berikutnya
	} else {
		fmt.Println("Token tidak valid")
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Unauthorized: Invalid token",
		})
	}
}

func (a *AuthMiddlewareStruct) CheckPermission(requiredPermission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Ambil userID dari context (hasil dari AuthMiddleware)
		userID, ok := c.Locals("userID").(float64) // Sesuaikan tipe datanya
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: Invalid user ID",
			})
		}

		// Query untuk mendapatkan daftar permissions berdasarkan userID
		var user models.User
		if err := a.DB.Debug().Preload("Roles.Permissions").First(&user, uint(userID)).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"message": "Forbidden: User not found",
			})
		}

		// Simpan semua permissions yang dimiliki user
		userPermissions := map[string]bool{}
		for _, role := range user.Roles {
			for _, perm := range role.Permissions {
				userPermissions[perm.Name] = true
			}
		}

		// fmt.Println("userPermissions: ", userPermissions)
		// fmt.Println("requiredPermission: ", requiredPermission)
		// fmt.Printf("userPermissions: %+v\n", userPermissions)
		// fmt.Printf("requiredPermission: '%s' (length: %d)\n", requiredPermission, len(requiredPermission))

		// // Cek apakah key benar-benar ada dalam map
		// if val, exists := userPermissions[requiredPermission]; exists {
		// 	fmt.Println("Permission found:", val)
		// } else {
		// 	fmt.Println("Permission NOT found!")
		// }

		// Cek apakah permission yang dibutuhkan ada dalam daftar userPermissions
		if _, exists := userPermissions[requiredPermission]; !exists {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"message": "Forbidden: You do not have permission",
			})
		}

		return c.Next() // Lanjut ke handler berikutnya
	}
}

func LoginMiddleware(ctx *fiber.Ctx) error {
	return ctx.Next()
}
