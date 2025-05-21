package controllers

import (
	"errors"
	"fiber-app/models"
	"os"
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

func (c *AuthController) Login(ctx *fiber.Ctx) error {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	// Parsing request body
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid input data",
		})
	}

	var mUser models.User
	// Cari user berdasarkan email
	result := c.DB.Where("email = ?", input.Email).First(&mUser)

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
		// Setting 1 Menit untuk testing
		// "exp": time.Now().Add(time.Second * 30).Unix(),
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to generate token",
		})
	}

	// Simpan token ke cookie
	ctx.Cookie(&fiber.Cookie{
		Name:    "token",
		Value:   tokenString,
		Expires: time.Now().Add(60 * time.Minute * 24), // Cookie berlaku 24 jam
		// Expires:  time.Now().Add(time.Second * 50),
		HTTPOnly: true,
		// SameSite: "Strict",
		// Secure:   true,
		// SameSite: "None",
		SameSite: "Lax",
		Secure:   false,
	})

	var menus []models.Menu
	errMenu := c.DB.
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
		"token":   tokenString,
		"user": fiber.Map{
			"id":       mUser.ID,
			"email":    mUser.Email,
			"username": mUser.Username,
			"name":     mUser.Name,
			"base_url": mUser.BaseRoute,
		},
		"menus": resultMenu,
	})
}

func (c *AuthController) Logout(ctx *fiber.Ctx) error {
	// Hapus token dari cookie
	ctx.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    "",
		Expires:  time.Now(),
		HTTPOnly: true,
		// Secure:   true,
		// SameSite: "Strict",

		SameSite: "Lax",
		Secure:   false,
	})

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Logout successful",
	})
}

func (c *AuthController) IsLoggedIn(ctx *fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "You are logged In",
	})
}
