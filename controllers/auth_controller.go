package controllers

import (
	"errors"
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/models"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
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

// func (c *AuthController) Logout(ctx *fiber.Ctx) error {
// 	// Hapus token dari cookie
// 	ctx.Cookie(config.GetTokenCookie(""))

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Logout successful",
// 	})
// }

func (c *AuthController) Logout(ctx *fiber.Ctx) error {
	sessionID, ok := ctx.Locals("sessionID").(string)
	if !ok || sessionID == "" {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid session",
		})
	}

	now := time.Now()

	// Update logout_at di login_logs
	result := c.DB.Model(&models.LoginLog{}).
		Where("session_id = ? AND logout_at IS NULL", sessionID).
		Update("logout_at", &now)

	if result.RowsAffected == 0 {
		// ini bukan error fatal, tapi penting untuk tahu
		// bisa karena double logout / token lama
		fmt.Println("Warning: No login log found to update logout_at for session_id:", sessionID)
	}

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
		Email    string `json:"email"`
		Password string `json:"password"`
		// BusinessUnit string `json:"business_unit"`
	}

	// Parsing request body
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
		})
	}

	if input.Email == "" || input.Password == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Missing required fields",
		})
	}

	ip, ua, browser, os, device := getClientInfo(ctx)
	now := time.Now()

	// default log FAILED
	log := models.LoginLog{
		Username:    input.Email,
		LoginAt:     &now,
		IPAddress:   ip,
		UserAgent:   ua,
		Browser:     browser,
		OS:          os,
		DeviceType:  device,
		LoginStatus: "FAILED",
		CreatedAt:   now,
	}

	db, err := database.GetDBConnection(config.DBUnit)
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
	result := db.Debug().Where("email = ? OR username = ?", input.Email, input.Email).First(&mUser)

	// Periksa jika user tidak ditemukan
	if result.Error != nil {

		reason := "USER_NOT_FOUND"
		log.FailureReason = &reason
		db.Create(&log)

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Invalid username or password",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": result.Error.Error(),
		})
	}

	// Verifikasi password (contoh sederhana, sebaiknya gunakan bcrypt)
	// if mUser.Password != input.Password {
	// 	return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
	// 		"message": "Invalid password",
	// 	})
	// }

	// cek password
	if bcrypt.CompareHashAndPassword(
		[]byte(mUser.Password),
		[]byte(input.Password),
	) != nil {
		reason := "WRONG_PASSWORD"
		uid := uint64(mUser.ID)
		log.UserID = &uid
		log.FailureReason = &reason
		db.Create(&log)

		return ctx.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
	}

	// === LOGIN SUCCESS ===
	sessionID := uuid.NewString()
	uid := uint64(mUser.ID)
	log.UserID = &uid
	log.LoginStatus = "SUCCESS"
	log.SessionID = sessionID
	log.FailureReason = nil

	db.Create(&log)

	// Buat access token JWT
	access_token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    mUser.ID,
		"session_id": sessionID,
		"exp":        time.Now().Add(time.Hour * 24).Unix(), // Token berlaku 24 jam
		"unit":       config.DBUnit,
		"jti":        uuid.NewString(),
	})

	// Buat refresh token JWT
	refresh_token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": mUser.ID,
		"unit":    config.DBUnit,
		"exp":     time.Now().Add(time.Hour * 24).Unix(), // Token berlaku 24 jam
		"jti":     uuid.NewString(),
	})

	accesTokenString, err := access_token.SignedString([]byte(config.JWTSecret))
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to generate token",
		})
	}

	refreshTokenString, err := refresh_token.SignedString([]byte(config.JWTSecret))
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to generate token",
		})
	}

	// Simpan refresh token ke cookie
	ctx.Cookie(config.GetTokenCookie(refreshTokenString))

	var permissionIDs []uint

	errPermission := db.
		Table("permissions").
		Select("permissions.id").
		Joins("JOIN role_permissions rp ON rp.permission_id = permissions.id").
		Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").
		Where("ur.user_id = ?", mUser.ID).
		Group("permissions.id").
		Pluck("permissions.id", &permissionIDs).Error
	if errPermission != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errPermission.Error(), "message": "Failed to get permission"})
	}

	var menus []models.Menu
	// errMenu := db.
	// 	Preload("Children").
	// 	Where("parent_id IS NULL").
	// 	Order("menu_order asc").
	// 	Find(&menus).Error
	// if errMenu != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errMenu.Error()})
	// }
	errMenu := db.
		Model(&models.Menu{}).
		Joins("JOIN menu_permissions mp ON mp.menu_id = menus.id").
		Where("mp.permission_id IN ?", permissionIDs).
		Where("menus.parent_id IS NULL").
		Preload("Children", func(tx *gorm.DB) *gorm.DB {
			return tx.
				Joins("JOIN menu_permissions mp2 ON mp2.menu_id = menus.id").
				Where("mp2.permission_id IN ?", permissionIDs).
				Order("menu_order asc")
		}).
		Order("menu_order asc").
		Find(&menus).Error
	if errMenu != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errMenu.Error(), "message": "Failed to get menus"})
	}

	// Map ke bentuk frontend
	var resultMenu []map[string]interface{}
	for _, menu := range menus {

		sort.Slice(menu.Children, func(i, j int) bool {
			return menu.Children[i].MenuOrder < menu.Children[j].MenuOrder
		})

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
		"x_token": accesTokenString,
		"user": fiber.Map{
			"id":       mUser.ID,
			"email":    mUser.Email,
			"username": mUser.Username,
			"name":     mUser.Name,
			"base_url": mUser.BaseRoute,
			"unit":     config.DBUnit,
		},
		"menus": resultMenu,
	})
}

func RefreshToken(ctx *fiber.Ctx) error {
	// Ambil cookie "refresh_token"
	tokenString := ctx.Cookies("refresh_token")
	if tokenString == "" {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Unauthorized - refresh token not found",
		})
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.JWTSecret), nil
	})

	if err != nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Unauthorized",
		})
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Generate new token
		newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"userID": claims["userID"],
			"unit":   claims["unit"],
			"exp":    time.Now().Add(time.Second * 15).Unix(),
			// "exp":    time.Now().Add(time.Hour * 24).Unix(), // Token berlaku 24 jam
		})
		newTokenString, err := newToken.SignedString([]byte(config.JWTSecret))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to generate token",
			})
		}

		// Simpan token ke cookie
		// ctx.Cookie(config.GetTokenCookie(newTokenString))

		fmt.Println("newTokenString: ", newTokenString)

		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
			"success":      true,
			"message":      "Token refreshed successfully",
			"access_token": newTokenString,
		})
	}

	return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"message": "Unauthorized",
	})
}

func getClientInfo(ctx *fiber.Ctx) (ip, ua, browser, os, device string) {
	ip = ctx.IP()
	ua = ctx.Get("User-Agent")

	uaLower := strings.ToLower(ua)

	switch {
	case strings.Contains(uaLower, "chrome"):
		browser = "Chrome"
	case strings.Contains(uaLower, "firefox"):
		browser = "Firefox"
	case strings.Contains(uaLower, "safari"):
		browser = "Safari"
	default:
		browser = "Unknown"
	}

	switch {
	case strings.Contains(uaLower, "windows"):
		os = "Windows"
	case strings.Contains(uaLower, "android"):
		os = "Android"
	case strings.Contains(uaLower, "iphone"):
		os = "iOS"
	case strings.Contains(uaLower, "linux"):
		os = "Linux"
	default:
		os = "Unknown"
	}

	if strings.Contains(uaLower, "mobile") {
		device = "MOBILE"
	} else {
		device = "DESKTOP"
	}

	return
}
