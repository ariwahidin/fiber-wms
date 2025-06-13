package controllers

import (
	"errors"
	"fiber-app/config"
	"fiber-app/controllers/configurations"
	"fiber-app/models"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type AuthController struct {
	DB *gorm.DB
}

func NewAuthController(DB *gorm.DB) *AuthController {
	return &AuthController{DB: DB}
}

// func (c *AuthController) Login(ctx *fiber.Ctx) error {
// 	var input struct {
// 		Email        string `json:"email"`
// 		Password     string `json:"password"`
// 		BusinessUnit string `json:"business_unit"`
// 	}

// 	// Parsing request body
// 	if err := ctx.BodyParser(&input); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"message": "Invalid request",
// 		})
// 	}

// 	// var db *gorm.DB
// 	// switch input.BusinessUnit {
// 	// case "Asics":
// 	// 	db = config.DBAsics
// 	// case "Mein":
// 	// 	db = config.DBMein
// 	// default:
// 	// 	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 	// 		"message": "Invalid Business Unit",
// 	// 	})
// 	// }

// 	// fmt.Println("Input data: ", input)

// 	// return nil

// 	var mUser models.User
// 	// Cari user berdasarkan email
// 	result := c.DB.Where("email = ?", input.Email).First(&mUser)

// 	// Periksa jika user tidak ditemukan
// 	if result.Error != nil {
// 		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 				"message": "Invalid email or password",
// 			})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": result.Error.Error(),
// 		})
// 	}

// 	// Verifikasi password (contoh sederhana, sebaiknya gunakan bcrypt)
// 	if mUser.Password != input.Password {
// 		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 			"message": "Invalid password",
// 		})
// 	}

// 	// Buat token JWT
// 	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
// 		"userID": mUser.ID,
// 		"exp":    time.Now().Add(time.Hour * 24).Unix(), // Token berlaku 24 jam
// 		// Setting 1 Menit untuk testing
// 		// "exp": time.Now().Add(time.Second * 30).Unix(),
// 	})

// 	tokenString, err := token.SignedString([]byte(config.JWTSecret))
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": "Failed to generate token",
// 		})
// 	}

// 	// Simpan token ke cookie
// 	ctx.Cookie(config.GetTokenCookie(tokenString))

// 	var menus []models.Menu
// 	errMenu := c.DB.
// 		Preload("Children").
// 		Where("parent_id IS NULL").
// 		Order("menu_order asc").
// 		Find(&menus).Error
// 	if errMenu != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errMenu.Error()})
// 	}

// 	// Map ke bentuk frontend
// 	var resultMenu []map[string]interface{}
// 	for _, menu := range menus {
// 		children := []map[string]interface{}{}
// 		for _, child := range menu.Children {
// 			children = append(children, map[string]interface{}{
// 				"title": child.Name,
// 				"url":   child.Path,
// 			})
// 		}

// 		resultMenu = append(resultMenu, map[string]interface{}{
// 			"title": menu.Name,
// 			"url":   menu.Path,
// 			"icon":  menu.Icon, // pastikan icon-nya string, misalnya "InboxIcon"
// 			// "isActive": menu.IsActive, // boolean
// 			"isActive": true,
// 			"items":    children, // anak-anak menu
// 		})
// 	}

// 	// Return data user (opsional, jangan kirim password)
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Login successful",
// 		"token":   tokenString,
// 		"user": fiber.Map{
// 			"id":       mUser.ID,
// 			"email":    mUser.Email,
// 			"username": mUser.Username,
// 			"name":     mUser.Name,
// 			"base_url": mUser.BaseRoute,
// 		},
// 		"menus": resultMenu,
// 	})
// }

func (c *AuthController) Logout(ctx *fiber.Ctx) error {
	// Hapus token dari cookie
	ctx.Cookie(config.GetTokenCookie(""))

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Logout successful",
	})
}

// func (c *AuthController) IsLoggedIn(ctx *fiber.Ctx) error {
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "You are logged In",
// 	})
// }

func Login(ctx *fiber.Ctx) error {
	var input struct {
		Email        string `json:"email"`
		Password     string `json:"password"`
		BusinessUnit string `json:"business_unit"`
	}

	// Parsing request body
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
		})
	}

	if input.Email == "" || input.Password == "" || input.BusinessUnit == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Missing required fields",
		})
	}

	db, err := configurations.GetDBConnection(input.BusinessUnit)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to connect to database",
		})
	}

	// return nil

	// var db *gorm.DB
	// switch input.BusinessUnit {
	// case "Asics":
	// 	db = config.DBAsics
	// case "Mein":
	// 	db = config.DBMein
	// default:
	// 	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"message": "Invalid Business Unit",
	// 	})
	// }

	// fmt.Println("Input data: ", input)

	// return nil

	var mUser models.User
	// Cari user berdasarkan email
	result := db.Debug().Where("email = ?", input.Email).First(&mUser)

	// Periksa jika user tidak ditemukan
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Invalid email or password",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": result.Error.Error(),
		})
	}

	// Verifikasi password (contoh sederhana, sebaiknya gunakan bcrypt)
	if mUser.Password != input.Password {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Invalid password",
		})
	}

	// Buat token JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userID": mUser.ID,
		"exp":    time.Now().Add(time.Hour * 24).Unix(), // Token berlaku 24 jam
		"unit":   input.BusinessUnit,
		// Setting 1 Menit untuk testing
		// "exp": time.Now().Add(time.Second * 30).Unix(),
	})

	tokenString, err := token.SignedString([]byte(config.JWTSecret))
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to generate token",
		})
	}

	// Simpan token ke cookie
	ctx.Cookie(config.GetTokenCookie(tokenString))

	var menus []models.Menu
	errMenu := db.
		Preload("Children").
		Where("parent_id IS NULL").
		Order("menu_order asc").
		Find(&menus).Error
	if errMenu != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errMenu.Error()})
	}

	// Map ke bentuk frontend
	var resultMenu []map[string]interface{}
	for _, menu := range menus {
		children := []map[string]interface{}{}
		for _, child := range menu.Children {
			children = append(children, map[string]interface{}{
				"title": child.Name,
				"url":   child.Path,
			})
		}

		resultMenu = append(resultMenu, map[string]interface{}{
			"title": menu.Name,
			"url":   menu.Path,
			"icon":  menu.Icon, // pastikan icon-nya string, misalnya "InboxIcon"
			// "isActive": menu.IsActive, // boolean
			"isActive": true,
			"items":    children, // anak-anak menu
		})
	}

	// Return data user (opsional, jangan kirim password)
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Login successful",
		"x_token": tokenString,
		"user": fiber.Map{
			"id":       mUser.ID,
			"email":    mUser.Email,
			"username": mUser.Username,
			"name":     mUser.Name,
			"base_url": mUser.BaseRoute,
			"unit":     input.BusinessUnit,
		},
		"menus": resultMenu,
	})
}
