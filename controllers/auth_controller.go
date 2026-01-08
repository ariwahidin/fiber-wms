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

	var userSession models.UserSession

	userSessResult := c.DB.Where("session_id = ? AND is_active = ? AND expires_at > ?", sessionID, true, time.Now()).First(&userSession)
	if userSessResult.Error != nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid session",
		})
	}

	userSession.IsActive = false
	userSession.LastActivityAt = time.Now()
	c.DB.Save(&userSession)

	// Hapus token dari cookie
	ctx.Cookie(config.GetTokenCookie(""))

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Logout successful",
	})
}

func LoginConfirm(ctx *fiber.Ctx) error {
	type Req struct {
		ConflictID string `json:"conflict_id"`
	}

	var req Req
	if err := ctx.BodyParser(&req); err != nil || req.ConflictID == "" {
		return ctx.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "conflict_id required",
		})
	}

	db, err := database.GetDBConnection(config.DBUnit)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to connect to database",
		})
	}

	loginConflict := models.LoginConflict{}
	if err := db.Where("id = ?", req.ConflictID).First(&loginConflict).Error; err != nil {
		return ctx.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "conflict_id not found",
		})
	}

	mUser := models.User{}
	if err := db.Where("id = ?", loginConflict.UserID).First(&mUser).Error; err != nil {
		return ctx.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "user not found",
		})
	}

	db.Model(&models.UserSession{}).
		Where("user_id = ? AND is_active = 1", mUser.ID).
		Update("is_active", false)

	sessionID := uuid.New().String()
	ip, ua, _, _, device := getClientInfo(ctx)
	now := time.Now()

	newSession := models.UserSession{
		UserID:         uint64(mUser.ID),
		SessionID:      sessionID,
		IPAddress:      ip,
		UserAgent:      ua,
		IsActive:       true,
		DeviceID:       device,
		LastActivityAt: now,
		ExpiresAt:      now.Add(24 * time.Hour),
	}

	db.Create(&newSession)

	// seesionID := uuid.New().String()
	// ip, ua, browser, os, device := getClientInfo(ctx)
	// now := time.Now()

	return LoginSuccess(sessionID, mUser, ctx)

	// return ctx.Status(200).JSON(fiber.Map{
	// 	"success": true,
	// 	"message": "Login successful",
	// })
}

func Login(ctx *fiber.Ctx) error {

	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
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

	sessionID := uuid.New().String()
	ip, ua, browser, os, device := getClientInfo(ctx)
	now := time.Now()

	// default log FAILED
	log := models.LoginLog{
		SessionID:   sessionID,
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

	//===============================================================================
	//BEGIN IF CONFLICT
	//===============================================================================

	var active models.UserSession
	result = db.Where("user_id = ? AND is_active = ?", mUser.ID, true).First(&active)
	if result.Error == nil {
		// UPDATE
		// active.SessionID = sessionID
		// active.LastActivityAt = now
		// active.ExpiresAt = now.Add(time.Hour * 24)
		// db.Save(&active)

		var conflict models.LoginConflict
		conflict.ID = uuid.NewString()
		conflict.UserID = uint64(mUser.ID)
		conflict.ExpiresAt = now.Add(time.Hour * 24)
		db.Create(&conflict)

		return ctx.Status(409).JSON(fiber.Map{
			"success": false,
			"device":  device,
			"ip":      ip,
			"ua":      ua,
			"cid":     conflict.ID,
			"message": fmt.Sprintf("User already logged in on another device: %s, last activity at: %s", active.DeviceID, active.LastActivityAt.Format("2006-01-02 15:04:05")),
			"error":   fmt.Sprintf("User already logged in on another device: %s, last activity at: %s", active.DeviceID, active.LastActivityAt.Format("2006-01-02 15:04:05")),
		})
	} else {
		// INSERT
		active = models.UserSession{
			UserID:         uint64(mUser.ID),
			SessionID:      sessionID,
			DeviceID:       device,
			IPAddress:      ip,
			UserAgent:      ua,
			IsActive:       true,
			LastActivityAt: now,
			ExpiresAt:      now.Add(time.Hour * 24),
		}
		db.Create(&active)
	}

	//===============================================================================
	//END IF CONFLICT
	//===============================================================================

	return LoginSuccess(sessionID, mUser, ctx)

	// === LOGIN SUCCESS ===
	// sessionID := uuid.NewString()
	// uid := uint64(mUser.ID)

	// log.UserID = &uid
	// log.LoginStatus = "SUCCESS"
	// log.SessionID = sessionID
	// log.FailureReason = nil

	// db.Create(&log)

	// // Buat access token JWT
	// access_token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
	// 	"user_id":    mUser.ID,
	// 	"session_id": sessionID,
	// 	"exp":        time.Now().Add(time.Hour * 24).Unix(), // Token berlaku 24 jam
	// 	"unit":       config.DBUnit,
	// 	"jti":        uuid.NewString(),
	// })

	// // Buat refresh token JWT
	// refresh_token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
	// 	"user_id": mUser.ID,
	// 	"unit":    config.DBUnit,
	// 	"exp":     time.Now().Add(time.Hour * 24).Unix(), // Token berlaku 24 jam
	// 	"jti":     uuid.NewString(),
	// })

	// accesTokenString, err := access_token.SignedString([]byte(config.JWTSecret))
	// if err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
	// 		"message": "Failed to generate token",
	// 	})
	// }

	// refreshTokenString, err := refresh_token.SignedString([]byte(config.JWTSecret))
	// if err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
	// 		"message": "Failed to generate token",
	// 	})
	// }

	// // Simpan refresh token ke cookie
	// ctx.Cookie(config.GetTokenCookie(refreshTokenString))

	// var permissionIDs []uint

	// errPermission := db.
	// 	Table("permissions").
	// 	Select("permissions.id").
	// 	Joins("JOIN role_permissions rp ON rp.permission_id = permissions.id").
	// 	Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").
	// 	Where("ur.user_id = ?", mUser.ID).
	// 	Group("permissions.id").
	// 	Pluck("permissions.id", &permissionIDs).Error
	// if errPermission != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errPermission.Error(), "message": "Failed to get permission"})
	// }

	// var menus []models.Menu
	// errMenu := db.
	// 	Model(&models.Menu{}).
	// 	Joins("JOIN menu_permissions mp ON mp.menu_id = menus.id").
	// 	Where("mp.permission_id IN ?", permissionIDs).
	// 	Where("menus.parent_id IS NULL").
	// 	Preload("Children", func(tx *gorm.DB) *gorm.DB {
	// 		return tx.
	// 			Joins("JOIN menu_permissions mp2 ON mp2.menu_id = menus.id").
	// 			Where("mp2.permission_id IN ?", permissionIDs).
	// 			Order("menu_order asc")
	// 	}).
	// 	Order("menu_order asc").
	// 	Find(&menus).Error
	// if errMenu != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errMenu.Error(), "message": "Failed to get menus"})
	// }

	// // Map ke bentuk frontend
	// var resultMenu []map[string]interface{}
	// for _, menu := range menus {

	// 	sort.Slice(menu.Children, func(i, j int) bool {
	// 		return menu.Children[i].MenuOrder < menu.Children[j].MenuOrder
	// 	})

	// 	children := []map[string]interface{}{}
	// 	for _, child := range menu.Children {
	// 		children = append(children, map[string]interface{}{
	// 			"title": child.Name,
	// 			"url":   child.Path,
	// 		})
	// 	}

	// 	resultMenu = append(resultMenu, map[string]interface{}{
	// 		"title": menu.Name,
	// 		"url":   menu.Path,
	// 		"icon":  menu.Icon, // pastikan icon-nya string, misalnya "InboxIcon"
	// 		// "isActive": menu.IsActive, // boolean
	// 		"isActive": true,
	// 		"items":    children, // anak-anak menu
	// 	})
	// }

	// var userRole models.User

	// errUserRole := db.
	// 	Preload("Roles").
	// 	First(&userRole, mUser.ID).Error

	// if errUserRole != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
	// 		"error":   errUserRole.Error(),
	// 		"message": "Failed to get user",
	// 	})
	// }

	// // Return data user (opsional, jangan kirim password)
	// return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
	// 	"success": true,
	// 	"message": "Login successfully",
	// 	"x_token": accesTokenString,
	// 	"user": fiber.Map{
	// 		"id":       mUser.ID,
	// 		"email":    mUser.Email,
	// 		"username": mUser.Username,
	// 		"name":     mUser.Name,
	// 		"base_url": mUser.BaseRoute,
	// 		"unit":     config.DBUnit,
	// 		"roles":    userRole.Roles,
	// 	},
	// 	"menus": resultMenu,
	// })
}

func LoginSuccess(sessionID string, mUser models.User, ctx *fiber.Ctx) error {

	ip, ua, browser, os, device := getClientInfo(ctx)
	now := time.Now()

	db, err := database.GetDBConnection(config.DBUnit)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to connect to database",
		})
	}

	// === LOGIN SUCCESS ===
	// sessionID := uuid.NewString()
	uid := uint64(mUser.ID)
	log := models.LoginLog{}
	log.UserID = &uid
	log.Username = mUser.Username
	log.IPAddress = ip
	log.UserAgent = ua
	log.LoginAt = &now
	log.OS = os
	log.DeviceType = device
	log.Browser = browser
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

	var userRole models.User

	errUserRole := db.
		Preload("Roles").
		First(&userRole, mUser.ID).Error

	if errUserRole != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   errUserRole.Error(),
			"message": "Failed to get user",
		})
	}

	// Return data user (opsional, jangan kirim password)
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Login successfully",
		"x_token": accesTokenString,
		"user": fiber.Map{
			"id":       mUser.ID,
			"email":    mUser.Email,
			"username": mUser.Username,
			"name":     mUser.Name,
			"base_url": mUser.BaseRoute,
			"unit":     config.DBUnit,
			"roles":    userRole.Roles,
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

func GetSessionActive(ctx *fiber.Ctx) error {

	type Req struct {
		ConflictID string `json:"conflict_id"`
	}

	var req Req
	if err := ctx.BodyParser(&req); err != nil || req.ConflictID == "" {
		return ctx.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "conflict_id required",
		})
	}

	db, err := database.GetDBConnection(config.DBUnit)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to connect to database",
		})
	}

	loginConflict := models.LoginConflict{}
	if err := db.Where("id = ?", req.ConflictID).First(&loginConflict).Error; err != nil {
		return ctx.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "conflict_id not found",
		})
	}

	sessionActives := []models.UserSession{}
	if err := db.Where("user_id = ? AND is_active = ?", loginConflict.UserID, true).Find(&sessionActives).Error; err != nil {
		return ctx.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "conflict_id not found",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Session active",
		"data":    sessionActives,
	})
}

func (c *AuthController) IsLoggedIn(ctx *fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "User is logged in",
	})
}
