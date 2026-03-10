// package controllers

// import (
// 	"errors"
// 	"fiber-app/config"
// 	"fiber-app/database"
// 	"fiber-app/models"
// 	"fmt"
// 	"sort"
// 	"strings"
// 	"time"

// 	"github.com/gofiber/fiber/v2"
// 	"github.com/golang-jwt/jwt/v5"
// 	"github.com/google/uuid"
// 	"golang.org/x/crypto/bcrypt"
// 	"gorm.io/gorm"
// )

// type AuthController struct {
// 	DB *gorm.DB
// }

// func NewAuthController(DB *gorm.DB) *AuthController {
// 	return &AuthController{DB: DB}
// }

// func (c *AuthController) Logout(ctx *fiber.Ctx) error {
// 	sessionID, ok := ctx.Locals("sessionID").(string)
// 	if !ok || sessionID == "" {
// 		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 			"error": "invalid session",
// 		})
// 	}

// 	now := time.Now()

// 	// Update logout_at di login_logs
// 	result := c.DB.Model(&models.LoginLog{}).
// 		Where("session_id = ? AND logout_at IS NULL", sessionID).
// 		Update("logout_at", &now)

// 	if result.RowsAffected == 0 {
// 		// ini bukan error fatal, tapi penting untuk tahu
// 		// bisa karena double logout / token lama
// 		fmt.Println("Warning: No login log found to update logout_at for session_id:", sessionID)
// 	}

// 	var userSession models.UserSession

// 	userSessResult := c.DB.Where("session_id = ? AND is_active = ? AND expires_at > ?", sessionID, true, time.Now()).First(&userSession)
// 	if userSessResult.Error != nil {
// 		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 			"error": "invalid session",
// 		})
// 	}

// 	userSession.IsActive = false
// 	userSession.LastActivityAt = time.Now()
// 	c.DB.Save(&userSession)

// 	// Hapus token dari cookie
// 	ctx.Cookie(config.GetTokenCookie(""))

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Logout successful",
// 	})
// }

// func LoginConfirm(ctx *fiber.Ctx) error {
// 	type Req struct {
// 		ConflictID string `json:"conflict_id"`
// 	}

// 	var req Req
// 	if err := ctx.BodyParser(&req); err != nil || req.ConflictID == "" {
// 		return ctx.Status(400).JSON(fiber.Map{
// 			"success": false,
// 			"message": "conflict_id required",
// 		})
// 	}

// 	db, err := database.GetDBConnection(config.DBUnit)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": "Failed to connect to database",
// 		})
// 	}

// 	loginConflict := models.LoginConflict{}
// 	if err := db.Where("id = ?", req.ConflictID).First(&loginConflict).Error; err != nil {
// 		return ctx.Status(400).JSON(fiber.Map{
// 			"success": false,
// 			"message": "conflict_id not found",
// 		})
// 	}

// 	mUser := models.User{}
// 	if err := db.Where("id = ?", loginConflict.UserID).First(&mUser).Error; err != nil {
// 		return ctx.Status(400).JSON(fiber.Map{
// 			"success": false,
// 			"message": "user not found",
// 		})
// 	}

// 	db.Model(&models.UserSession{}).
// 		Where("user_id = ? AND is_active = 1", mUser.ID).
// 		Update("is_active", false)

// 	sessionID := uuid.New().String()
// 	ip, ua, _, _, device := getClientInfo(ctx)
// 	now := time.Now()

// 	newSession := models.UserSession{
// 		UserID:         uint64(mUser.ID),
// 		SessionID:      sessionID,
// 		IPAddress:      ip,
// 		UserAgent:      ua,
// 		IsActive:       true,
// 		DeviceID:       device,
// 		LastActivityAt: now,
// 		ExpiresAt:      now.Add(24 * time.Hour),
// 	}

// 	db.Create(&newSession)

// 	// seesionID := uuid.New().String()
// 	// ip, ua, browser, os, device := getClientInfo(ctx)
// 	// now := time.Now()

// 	return LoginSuccess(sessionID, mUser, ctx)

// 	// return ctx.Status(200).JSON(fiber.Map{
// 	// 	"success": true,
// 	// 	"message": "Login successful",
// 	// })
// }

// func Login(ctx *fiber.Ctx) error {

// 	var input struct {
// 		Email    string `json:"email"`
// 		Password string `json:"password"`
// 	}

// 	// Parsing request body
// 	if err := ctx.BodyParser(&input); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"message": "Invalid request",
// 		})
// 	}

// 	if input.Email == "" || input.Password == "" {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"message": "Missing required fields",
// 		})
// 	}

// 	sessionID := uuid.New().String()
// 	ip, ua, browser, os, device := getClientInfo(ctx)
// 	now := time.Now()

// 	// default log FAILED
// 	log := models.LoginLog{
// 		SessionID:   sessionID,
// 		Username:    input.Email,
// 		LoginAt:     &now,
// 		IPAddress:   ip,
// 		UserAgent:   ua,
// 		Browser:     browser,
// 		OS:          os,
// 		DeviceType:  device,
// 		LoginStatus: "FAILED",
// 		CreatedAt:   now,
// 	}

// 	db, err := database.GetDBConnection(config.DBUnit)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": "Failed to connect to database",
// 		})
// 	}

// 	var mUser models.User
// 	// Cari user berdasarkan email
// 	result := db.Debug().Where("email = ? OR username = ?", input.Email, input.Email).First(&mUser)

// 	// Periksa jika user tidak ditemukan
// 	if result.Error != nil {

// 		reason := "USER_NOT_FOUND"
// 		log.FailureReason = &reason
// 		db.Create(&log)

// 		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 				"message": "Invalid username or password",
// 			})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": result.Error.Error(),
// 		})
// 	}

// 	// cek password
// 	if bcrypt.CompareHashAndPassword(
// 		[]byte(mUser.Password),
// 		[]byte(input.Password),
// 	) != nil {
// 		reason := "WRONG_PASSWORD"
// 		uid := uint64(mUser.ID)
// 		log.UserID = &uid
// 		log.FailureReason = &reason
// 		db.Create(&log)

// 		return ctx.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
// 	}

// 	//===============================================================================
// 	//BEGIN IF CONFLICT
// 	//===============================================================================

// 	var active models.UserSession
// 	result = db.Where("user_id = ? AND is_active = ?", mUser.ID, true).First(&active)
// 	if result.Error == nil {
// 		// UPDATE
// 		// active.SessionID = sessionID
// 		// active.LastActivityAt = now
// 		// active.ExpiresAt = now.Add(time.Hour * 24)
// 		// db.Save(&active)

// 		var conflict models.LoginConflict
// 		conflict.ID = uuid.NewString()
// 		conflict.UserID = uint64(mUser.ID)
// 		conflict.ExpiresAt = now.Add(time.Hour * 24)
// 		db.Create(&conflict)

// 		return ctx.Status(409).JSON(fiber.Map{
// 			"success": false,
// 			"device":  device,
// 			"ip":      ip,
// 			"ua":      ua,
// 			"cid":     conflict.ID,
// 			"message": fmt.Sprintf("User already logged in on another device: %s, last activity at: %s", active.DeviceID, active.LastActivityAt.Format("2006-01-02 15:04:05")),
// 			"error":   fmt.Sprintf("User already logged in on another device: %s, last activity at: %s", active.DeviceID, active.LastActivityAt.Format("2006-01-02 15:04:05")),
// 		})
// 	} else {
// 		// INSERT
// 		active = models.UserSession{
// 			UserID:         uint64(mUser.ID),
// 			SessionID:      sessionID,
// 			DeviceID:       device,
// 			IPAddress:      ip,
// 			UserAgent:      ua,
// 			IsActive:       true,
// 			LastActivityAt: now,
// 			ExpiresAt:      now.Add(time.Hour * 24),
// 		}
// 		db.Create(&active)
// 	}

// 	//===============================================================================
// 	//END IF CONFLICT
// 	//===============================================================================

// 	return LoginSuccess(sessionID, mUser, ctx)

// }

// // func LoginSuccess(sessionID string, mUser models.User, ctx *fiber.Ctx) error {

// // 	ip, ua, browser, os, device := getClientInfo(ctx)
// // 	now := time.Now()

// // 	db, err := database.GetDBConnection(config.DBUnit)
// // 	if err != nil {
// // 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// // 			"message": "Failed to connect to database",
// // 		})
// // 	}

// // 	// === LOGIN SUCCESS ===
// // 	// sessionID := uuid.NewString()
// // 	uid := uint64(mUser.ID)
// // 	log := models.LoginLog{}
// // 	log.UserID = &uid
// // 	log.Username = mUser.Username
// // 	log.IPAddress = ip
// // 	log.UserAgent = ua
// // 	log.LoginAt = &now
// // 	log.OS = os
// // 	log.DeviceType = device
// // 	log.Browser = browser
// // 	log.LoginStatus = "SUCCESS"
// // 	log.SessionID = sessionID
// // 	log.FailureReason = nil

// // 	db.Create(&log)

// // 	// Buat access token JWT
// // 	access_token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
// // 		"user_id":    mUser.ID,
// // 		"session_id": sessionID,
// // 		"exp":        time.Now().Add(time.Hour * 24).Unix(), // Token berlaku 24 jam
// // 		"unit":       config.DBUnit,
// // 		"jti":        uuid.NewString(),
// // 	})

// // 	// Buat refresh token JWT
// // 	refresh_token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
// // 		"user_id": mUser.ID,
// // 		"unit":    config.DBUnit,
// // 		"exp":     time.Now().Add(time.Hour * 24).Unix(), // Token berlaku 24 jam
// // 		"jti":     uuid.NewString(),
// // 	})

// // 	accesTokenString, err := access_token.SignedString([]byte(config.JWTSecret))
// // 	if err != nil {
// // 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// // 			"message": "Failed to generate token",
// // 		})
// // 	}

// // 	refreshTokenString, err := refresh_token.SignedString([]byte(config.JWTSecret))
// // 	if err != nil {
// // 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// // 			"message": "Failed to generate token",
// // 		})
// // 	}

// // 	// Simpan refresh token ke cookie
// // 	ctx.Cookie(config.GetTokenCookie(refreshTokenString))

// // 	var permissionIDs []uint

// // 	errPermission := db.
// // 		Table("permissions").
// // 		Select("permissions.id").
// // 		Joins("JOIN role_permissions rp ON rp.permission_id = permissions.id").
// // 		Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").
// // 		Where("ur.user_id = ?", mUser.ID).
// // 		Group("permissions.id").
// // 		Pluck("permissions.id", &permissionIDs).Error
// // 	if errPermission != nil {
// // 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errPermission.Error(), "message": "Failed to get permission"})
// // 	}

// // 	var menus []models.Menu
// // 	errMenu := db.
// // 		Model(&models.Menu{}).
// // 		Joins("JOIN menu_permissions mp ON mp.menu_id = menus.id").
// // 		Where("mp.permission_id IN ?", permissionIDs).
// // 		Where("menus.parent_id IS NULL").
// // 		Preload("Children", func(tx *gorm.DB) *gorm.DB {
// // 			return tx.
// // 				Joins("JOIN menu_permissions mp2 ON mp2.menu_id = menus.id").
// // 				Where("mp2.permission_id IN ?", permissionIDs).
// // 				Order("menu_order asc")
// // 		}).
// // 		Order("menu_order asc").
// // 		Find(&menus).Error
// // 	if errMenu != nil {
// // 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errMenu.Error(), "message": "Failed to get menus"})
// // 	}

// // 	// Map ke bentuk frontend
// // 	var resultMenu []map[string]interface{}
// // 	for _, menu := range menus {

// // 		sort.Slice(menu.Children, func(i, j int) bool {
// // 			return menu.Children[i].MenuOrder < menu.Children[j].MenuOrder
// // 		})

// // 		children := []map[string]interface{}{}
// // 		for _, child := range menu.Children {
// // 			children = append(children, map[string]interface{}{
// // 				"title": child.Name,
// // 				"url":   child.Path,
// // 			})
// // 		}

// // 		resultMenu = append(resultMenu, map[string]interface{}{
// // 			"title": menu.Name,
// // 			"url":   menu.Path,
// // 			"icon":  menu.Icon, // pastikan icon-nya string, misalnya "InboxIcon"
// // 			// "isActive": menu.IsActive, // boolean
// // 			"isActive": true,
// // 			"items":    children, // anak-anak menu
// // 		})
// // 	}

// // 	var userRole models.User

// // 	errUserRole := db.
// // 		Preload("Roles").
// // 		First(&userRole, mUser.ID).Error

// // 	if errUserRole != nil {
// // 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// // 			"error":   errUserRole.Error(),
// // 			"message": "Failed to get user",
// // 		})
// // 	}

// // 	// Return data user (opsional, jangan kirim password)
// // 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// // 		"success": true,
// // 		"message": "Login successfully",
// // 		"x_token": accesTokenString,
// // 		"user": fiber.Map{
// // 			"id":       mUser.ID,
// // 			"email":    mUser.Email,
// // 			"username": mUser.Username,
// // 			"name":     mUser.Name,
// // 			"base_url": mUser.BaseRoute,
// // 			"unit":     config.DBUnit,
// // 			"roles":    userRole.Roles,
// // 		},
// // 		"menus": resultMenu,
// // 	})

// // }

// func LoginSuccess(sessionID string, mUser models.User, ctx *fiber.Ctx) error {
// 	db, err := database.GetDBConnection(config.DBUnit)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": "Failed to connect to database",
// 		})
// 	}

// 	// Log login activity
// 	if err := createLoginLog(db, sessionID, mUser, ctx); err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": "Failed to record login log",
// 		})
// 	}

// 	// Generate tokens
// 	accessTokenString, refreshTokenString, err := generateTokenPair(mUser.ID, sessionID)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": err.Error(),
// 		})
// 	}

// 	ctx.Cookie(config.GetTokenCookie(refreshTokenString))

// 	// Fetch permissions, menus, roles in parallel
// 	type result struct {
// 		permissionIDs []uint
// 		menus         []models.Menu
// 		userWithRoles models.User
// 		err           error
// 	}

// 	permCh := make(chan result, 1)
// 	menuCh := make(chan result, 1)
// 	roleCh := make(chan result, 1)

// 	go func() {
// 		var ids []uint
// 		err := db.Table("permissions").
// 			Select("permissions.id").
// 			Joins("JOIN role_permissions rp ON rp.permission_id = permissions.id").
// 			Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").
// 			Where("ur.user_id = ?", mUser.ID).
// 			Group("permissions.id").
// 			Pluck("permissions.id", &ids).Error
// 		permCh <- result{permissionIDs: ids, err: err}
// 	}()

// 	permResult := <-permCh
// 	if permResult.err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error":   permResult.err.Error(),
// 			"message": "Failed to get permissions",
// 		})
// 	}

// 	go func() {
// 		var menus []models.Menu
// 		err := db.Model(&models.Menu{}).
// 			Joins("JOIN menu_permissions mp ON mp.menu_id = menus.id").
// 			Where("mp.permission_id IN ?", permResult.permissionIDs).
// 			Where("menus.parent_id IS NULL").
// 			Preload("Children", func(tx *gorm.DB) *gorm.DB {
// 				return tx.
// 					Joins("JOIN menu_permissions mp2 ON mp2.menu_id = menus.id").
// 					Where("mp2.permission_id IN ?", permResult.permissionIDs).
// 					Order("menu_order asc")
// 			}).
// 			Order("menu_order asc").
// 			Find(&menus).Error
// 		menuCh <- result{menus: menus, err: err}
// 	}()

// 	go func() {
// 		var u models.User
// 		err := db.Preload("Roles").First(&u, mUser.ID).Error
// 		roleCh <- result{userWithRoles: u, err: err}
// 	}()

// 	menuResult := <-menuCh
// 	if menuResult.err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error":   menuResult.err.Error(),
// 			"message": "Failed to get menus",
// 		})
// 	}

// 	roleResult := <-roleCh
// 	if roleResult.err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error":   roleResult.err.Error(),
// 			"message": "Failed to get user roles",
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Login successfully",
// 		"x_token": accessTokenString,
// 		"user": fiber.Map{
// 			"id":       mUser.ID,
// 			"email":    mUser.Email,
// 			"username": mUser.Username,
// 			"name":     mUser.Name,
// 			"base_url": mUser.BaseRoute,
// 			"unit":     config.DBUnit,
// 			"roles":    roleResult.userWithRoles.Roles,
// 		},
// 		"menus": buildMenuResponse(menuResult.menus),
// 	})
// }

// // --- Helpers ---

// func createLoginLog(db *gorm.DB, sessionID string, mUser models.User, ctx *fiber.Ctx) error {
// 	ip, ua, browser, os, device := getClientInfo(ctx)
// 	now := time.Now()
// 	uid := uint64(mUser.ID)

// 	log := models.LoginLog{
// 		UserID:        &uid,
// 		Username:      mUser.Username,
// 		IPAddress:     ip,
// 		UserAgent:     ua,
// 		LoginAt:       &now,
// 		OS:            os,
// 		DeviceType:    device,
// 		Browser:       browser,
// 		LoginStatus:   "SUCCESS",
// 		SessionID:     sessionID,
// 		FailureReason: nil,
// 	}

// 	return db.Create(&log).Error
// }

// func generateTokenPair(userID interface{}, sessionID string, permissions []string) (accessToken, refreshToken string, err error) {
// 	expiry := time.Now().Add(24 * time.Hour).Unix()

// 	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
// 		"user_id":     userID,
// 		"session_id":  sessionID,
// 		"permissions": permissions,
// 		"exp":         expiry,
// 		"unit":        config.DBUnit,
// 		"jti":         uuid.NewString(),
// 	})

// 	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
// 		"user_id": userID,
// 		"unit":    config.DBUnit,
// 		"exp":     expiry,
// 		"jti":     uuid.NewString(),
// 	})

// 	secret := []byte(config.JWTSecret)

// 	accessToken, err = access.SignedString(secret)
// 	if err != nil {
// 		return "", "", fmt.Errorf("failed to generate access token")
// 	}

// 	refreshToken, err = refresh.SignedString(secret)
// 	if err != nil {
// 		return "", "", fmt.Errorf("failed to generate refresh token")
// 	}

// 	return accessToken, refreshToken, nil
// }

// func buildMenuResponse(menus []models.Menu) []map[string]interface{} {
// 	result := make([]map[string]interface{}, 0, len(menus))

// 	for _, menu := range menus {
// 		sort.Slice(menu.Children, func(i, j int) bool {
// 			return menu.Children[i].MenuOrder < menu.Children[j].MenuOrder
// 		})

// 		children := make([]map[string]interface{}, 0, len(menu.Children))
// 		for _, child := range menu.Children {
// 			children = append(children, map[string]interface{}{
// 				"title": child.Name,
// 				"url":   child.Path,
// 			})
// 		}

// 		result = append(result, map[string]interface{}{
// 			"title":    menu.Name,
// 			"url":      menu.Path,
// 			"icon":     menu.Icon,
// 			"isActive": true,
// 			"items":    children,
// 		})
// 	}

// 	return result
// }

// func RefreshToken(ctx *fiber.Ctx) error {
// 	// Ambil cookie "refresh_token"
// 	tokenString := ctx.Cookies("refresh_token")
// 	if tokenString == "" {
// 		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 			"message": "Unauthorized - refresh token not found",
// 		})
// 	}

// 	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
// 		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
// 			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
// 		}
// 		return []byte(config.JWTSecret), nil
// 	})

// 	if err != nil {
// 		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 			"message": "Unauthorized",
// 		})
// 	}

// 	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
// 		// Generate new token
// 		newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
// 			"userID": claims["userID"],
// 			"unit":   claims["unit"],
// 			"exp":    time.Now().Add(time.Second * 15).Unix(),
// 			// "exp":    time.Now().Add(time.Hour * 24).Unix(), // Token berlaku 24 jam
// 		})
// 		newTokenString, err := newToken.SignedString([]byte(config.JWTSecret))
// 		if err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"message": "Failed to generate token",
// 			})
// 		}

// 		// Simpan token ke cookie
// 		// ctx.Cookie(config.GetTokenCookie(newTokenString))

// 		fmt.Println("newTokenString: ", newTokenString)

// 		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 			"success":      true,
// 			"message":      "Token refreshed successfully",
// 			"access_token": newTokenString,
// 		})
// 	}

// 	return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 		"message": "Unauthorized",
// 	})
// }

// func getClientInfo(ctx *fiber.Ctx) (ip, ua, browser, os, device string) {
// 	ip = ctx.IP()
// 	ua = ctx.Get("User-Agent")

// 	uaLower := strings.ToLower(ua)

// 	switch {
// 	case strings.Contains(uaLower, "chrome"):
// 		browser = "Chrome"
// 	case strings.Contains(uaLower, "firefox"):
// 		browser = "Firefox"
// 	case strings.Contains(uaLower, "safari"):
// 		browser = "Safari"
// 	default:
// 		browser = "Unknown"
// 	}

// 	switch {
// 	case strings.Contains(uaLower, "windows"):
// 		os = "Windows"
// 	case strings.Contains(uaLower, "android"):
// 		os = "Android"
// 	case strings.Contains(uaLower, "iphone"):
// 		os = "iOS"
// 	case strings.Contains(uaLower, "linux"):
// 		os = "Linux"
// 	default:
// 		os = "Unknown"
// 	}

// 	if strings.Contains(uaLower, "mobile") {
// 		device = "MOBILE"
// 	} else {
// 		device = "DESKTOP"
// 	}

// 	return
// }

// func GetSessionActive(ctx *fiber.Ctx) error {

// 	type Req struct {
// 		ConflictID string `json:"conflict_id"`
// 	}

// 	var req Req
// 	if err := ctx.BodyParser(&req); err != nil || req.ConflictID == "" {
// 		return ctx.Status(400).JSON(fiber.Map{
// 			"success": false,
// 			"message": "conflict_id required",
// 		})
// 	}

// 	db, err := database.GetDBConnection(config.DBUnit)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": "Failed to connect to database",
// 		})
// 	}

// 	loginConflict := models.LoginConflict{}
// 	if err := db.Where("id = ?", req.ConflictID).First(&loginConflict).Error; err != nil {
// 		return ctx.Status(400).JSON(fiber.Map{
// 			"success": false,
// 			"message": "conflict_id not found",
// 		})
// 	}

// 	sessionActives := []models.UserSession{}
// 	if err := db.Where("user_id = ? AND is_active = ?", loginConflict.UserID, true).Find(&sessionActives).Error; err != nil {
// 		return ctx.Status(400).JSON(fiber.Map{
// 			"success": false,
// 			"message": "conflict_id not found",
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Session active",
// 		"data":    sessionActives,
// 	})
// }

// func (c *AuthController) IsLoggedIn(ctx *fiber.Ctx) error {
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "User is logged in",
// 	})
// }

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

// ============================================================
// Controller
// ============================================================

type AuthController struct {
	DB *gorm.DB
}

func NewAuthController(DB *gorm.DB) *AuthController {
	return &AuthController{DB: DB}
}

// ============================================================
// Handlers
// ============================================================

func (c *AuthController) IsLoggedIn(ctx *fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "User is logged in",
	})
}

func (c *AuthController) Logout(ctx *fiber.Ctx) error {
	sessionID, ok := ctx.Locals("sessionID").(string)
	if !ok || sessionID == "" {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid session",
		})
	}

	now := time.Now()

	result := c.DB.Model(&models.LoginLog{}).
		Where("session_id = ? AND logout_at IS NULL", sessionID).
		Update("logout_at", &now)
	if result.RowsAffected == 0 {
		fmt.Println("Warning: no login log found for session_id:", sessionID)
	}

	var session models.UserSession
	if err := c.DB.Where("session_id = ? AND is_active = ? AND expires_at > ?", sessionID, true, now).
		First(&session).Error; err != nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid session",
		})
	}

	session.IsActive = false
	session.LastActivityAt = now
	c.DB.Save(&session)

	ctx.Cookie(config.GetTokenCookie(""))

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Logout successful",
	})
}

func Login(ctx *fiber.Ctx) error {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}
	if input.Email == "" || input.Password == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Missing required fields"})
	}

	db, err := database.GetDBConnection(config.DBUnit)
	if err != nil {
		return errInternalDB(ctx)
	}

	ip, ua, browser, os, device := getClientInfo(ctx)
	now := time.Now()
	sessionID := uuid.New().String()

	// Default log entry (FAILED) — updated to SUCCESS inside LoginSuccess
	failLog := models.LoginLog{
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

	var mUser models.User
	if err := db.Where("email = ? OR username = ?", input.Email, input.Email).First(&mUser).Error; err != nil {
		reason := "USER_NOT_FOUND"
		failLog.FailureReason = &reason
		db.Create(&failLog)

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Invalid username or password"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(mUser.Password), []byte(input.Password)); err != nil {
		reason := "WRONG_PASSWORD"
		uid := uint64(mUser.ID)
		failLog.UserID = &uid
		failLog.FailureReason = &reason
		db.Create(&failLog)
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}

	// Check concurrent session conflict
	var active models.UserSession
	if err := db.Where("user_id = ? AND is_active = ?", mUser.ID, true).First(&active).Error; err == nil {
		conflict := models.LoginConflict{
			ID:        uuid.NewString(),
			UserID:    uint64(mUser.ID),
			ExpiresAt: now.Add(24 * time.Hour),
		}
		db.Create(&conflict)

		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"device":  device,
			"ip":      ip,
			"ua":      ua,
			"cid":     conflict.ID,
			"message": fmt.Sprintf("User already logged in on another device: %s, last activity at: %s",
				active.DeviceID, active.LastActivityAt.Format("2006-01-02 15:04:05")),
			"error": fmt.Sprintf("User already logged in on another device: %s, last activity at: %s",
				active.DeviceID, active.LastActivityAt.Format("2006-01-02 15:04:05")),
		})
	}

	// New session
	db.Create(&models.UserSession{
		UserID:         uint64(mUser.ID),
		SessionID:      sessionID,
		DeviceID:       device,
		IPAddress:      ip,
		UserAgent:      ua,
		IsActive:       true,
		LastActivityAt: now,
		ExpiresAt:      now.Add(24 * time.Hour),
	})

	return LoginSuccess(sessionID, mUser, ctx)
}

func LoginConfirm(ctx *fiber.Ctx) error {
	var req struct {
		ConflictID string `json:"conflict_id"`
	}
	if err := ctx.BodyParser(&req); err != nil || req.ConflictID == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "conflict_id required",
		})
	}

	db, err := database.GetDBConnection(config.DBUnit)
	if err != nil {
		return errInternalDB(ctx)
	}

	var conflict models.LoginConflict
	if err := db.Where("id = ?", req.ConflictID).First(&conflict).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "conflict_id not found",
		})
	}

	var mUser models.User
	if err := db.Where("id = ?", conflict.UserID).First(&mUser).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "user not found",
		})
	}

	// Invalidate all existing sessions
	db.Model(&models.UserSession{}).
		Where("user_id = ? AND is_active = 1", mUser.ID).
		Update("is_active", false)

	ip, ua, _, _, device := getClientInfo(ctx)
	now := time.Now()
	sessionID := uuid.New().String()

	db.Create(&models.UserSession{
		UserID:         uint64(mUser.ID),
		SessionID:      sessionID,
		IPAddress:      ip,
		UserAgent:      ua,
		IsActive:       true,
		DeviceID:       device,
		LastActivityAt: now,
		ExpiresAt:      now.Add(24 * time.Hour),
	})

	return LoginSuccess(sessionID, mUser, ctx)
}

func GetSessionActive(ctx *fiber.Ctx) error {
	var req struct {
		ConflictID string `json:"conflict_id"`
	}
	if err := ctx.BodyParser(&req); err != nil || req.ConflictID == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "conflict_id required",
		})
	}

	db, err := database.GetDBConnection(config.DBUnit)
	if err != nil {
		return errInternalDB(ctx)
	}

	var conflict models.LoginConflict
	if err := db.Where("id = ?", req.ConflictID).First(&conflict).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "conflict_id not found",
		})
	}

	var sessions []models.UserSession
	if err := db.Where("user_id = ? AND is_active = ?", conflict.UserID, true).Find(&sessions).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "failed to fetch sessions",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Session active",
		"data":    sessions,
	})
}

func RefreshToken(ctx *fiber.Ctx) error {
	tokenString := ctx.Cookies("refresh_token")
	if tokenString == "" {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Unauthorized - refresh token not found",
		})
	}

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}

	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userID": claims["userID"],
		"unit":   claims["unit"],
		"exp":    time.Now().Add(15 * time.Second).Unix(),
	})

	newTokenString, err := newToken.SignedString([]byte(config.JWTSecret))
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to generate token"})
	}

	fmt.Println("newTokenString:", newTokenString)

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":      true,
		"message":      "Token refreshed successfully",
		"access_token": newTokenString,
	})
}

// ============================================================
// LoginSuccess
// ============================================================

func LoginSuccess(sessionID string, mUser models.User, ctx *fiber.Ctx) error {
	db, err := database.GetDBConnection(config.DBUnit)
	if err != nil {
		return errInternalDB(ctx)
	}

	if err := createLoginLog(db, sessionID, mUser, ctx); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to record login log",
		})
	}

	// Fetch permissionIDs dulu — dibutuhkan oleh fetchMenus
	permissionIDs, err := fetchPermissionIDs(db, mUser.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   err.Error(),
			"message": "Failed to get permissions",
		})
	}

	// Fetch menus, permission strings, dan roles secara concurrent
	type menuResult struct {
		menus []models.Menu
		err   error
	}
	type roleResult struct {
		user models.User
		err  error
	}
	type permStringResult struct {
		perms []string
		err   error
	}

	menuCh := make(chan menuResult, 1)
	roleCh := make(chan roleResult, 1)
	permStrCh := make(chan permStringResult, 1)

	go func() {
		menus, err := fetchMenus(db, permissionIDs)
		menuCh <- menuResult{menus, err}
	}()

	go func() {
		var u models.User
		err := db.Preload("Roles").First(&u, mUser.ID).Error
		roleCh <- roleResult{u, err}
	}()

	go func() {
		perms, err := fetchPermissionStrings(db, mUser.ID)
		permStrCh <- permStringResult{perms, err}
	}()

	mr := <-menuCh
	if mr.err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   mr.err.Error(),
			"message": "Failed to get menus",
		})
	}

	rr := <-roleCh
	if rr.err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   rr.err.Error(),
			"message": "Failed to get user roles",
		})
	}

	pr := <-permStrCh
	if pr.err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   pr.err.Error(),
			"message": "Failed to get permission strings",
		})
	}

	// Build role names untuk superadmin bypass di middleware
	roleNames := make([]string, 0, len(rr.user.Roles))
	for _, r := range rr.user.Roles {
		roleNames = append(roleNames, r.Name)
	}

	accessToken, refreshToken, err := generateTokenPair(mUser.ID, sessionID, pr.perms, roleNames)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}
	ctx.Cookie(config.GetTokenCookie(refreshToken))

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Login successfully",
		"x_token": accessToken,
		"user": fiber.Map{
			"id":       mUser.ID,
			"email":    mUser.Email,
			"username": mUser.Username,
			"name":     mUser.Name,
			"base_url": mUser.BaseRoute,
			"unit":     config.DBUnit,
			"roles":    rr.user.Roles,
		},
		"permissions": pr.perms, // ["supplier:read", "inbound:create", ...]
		"menus":       buildMenuResponse(mr.menus),
	})
}

// ============================================================
// Private helpers
// ============================================================

// fetchPermissionIDs mengambil ID permission user — dipakai untuk filter menu
func fetchPermissionIDs(db *gorm.DB, userID uint) ([]uint, error) {
	var ids []uint
	err := db.Table("permissions").
		Select("permissions.id").
		Joins("JOIN role_permissions rp ON rp.permission_id = permissions.id").
		Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").
		Where("ur.user_id = ?", userID).
		Group("permissions.id").
		Pluck("permissions.id", &ids).Error
	return ids, err
}

// fetchPermissionStrings mengambil permission dalam format "resource:action"
// dipakai untuk inject ke JWT claims dan dikirim ke frontend
func fetchPermissionStrings(db *gorm.DB, userID uint) ([]string, error) {
	type row struct {
		Resource string
		Action   string
	}
	var rows []row
	err := db.Table("permissions").
		Select("permissions.resource, permissions.action").
		Joins("JOIN role_permissions rp ON rp.permission_id = permissions.id").
		Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").
		Where("ur.user_id = ? AND permissions.resource != '' AND permissions.action != ''", userID).
		Group("permissions.resource, permissions.action").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	perms := make([]string, 0, len(rows))
	for _, r := range rows {
		perms = append(perms, r.Resource+":"+r.Action)
	}
	return perms, nil
}

// fetchMenus mengambil menu parent + children berdasarkan permissionIDs
func fetchMenus(db *gorm.DB, permissionIDs []uint) ([]models.Menu, error) {
	var menus []models.Menu
	err := db.Model(&models.Menu{}).
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
	return menus, err
}

func createLoginLog(db *gorm.DB, sessionID string, mUser models.User, ctx *fiber.Ctx) error {
	ip, ua, browser, os, device := getClientInfo(ctx)
	now := time.Now()
	uid := uint64(mUser.ID)

	return db.Create(&models.LoginLog{
		UserID:        &uid,
		Username:      mUser.Username,
		IPAddress:     ip,
		UserAgent:     ua,
		LoginAt:       &now,
		OS:            os,
		DeviceType:    device,
		Browser:       browser,
		LoginStatus:   "SUCCESS",
		SessionID:     sessionID,
		FailureReason: nil,
	}).Error
}

// generateTokenPair membuat access token (berisi permissions & roles) dan refresh token
func generateTokenPair(userID interface{}, sessionID string, permissions []string, roles []string) (accessToken, refreshToken string, err error) {
	expiry := time.Now().Add(24 * time.Hour).Unix()
	secret := []byte(config.JWTSecret)

	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":     userID,
		"session_id":  sessionID,
		"permissions": permissions, // ["supplier:read", "inbound:create", ...]
		"roles":       roles,       // ["SUPERADMIN"] — untuk bypass di middleware
		"exp":         expiry,
		"unit":        config.DBUnit,
		"jti":         uuid.NewString(),
	})

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"unit":    config.DBUnit,
		"exp":     expiry,
		"jti":     uuid.NewString(),
	})

	if accessToken, err = access.SignedString(secret); err != nil {
		return "", "", fmt.Errorf("failed to generate access token")
	}
	if refreshToken, err = refresh.SignedString(secret); err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token")
	}
	return
}

func buildMenuResponse(menus []models.Menu) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(menus))
	for _, menu := range menus {
		sort.Slice(menu.Children, func(i, j int) bool {
			return menu.Children[i].MenuOrder < menu.Children[j].MenuOrder
		})

		children := make([]map[string]interface{}, 0, len(menu.Children))
		for _, child := range menu.Children {
			children = append(children, map[string]interface{}{
				"title": child.Name,
				"url":   child.Path,
			})
		}

		result = append(result, map[string]interface{}{
			"title":    menu.Name,
			"url":      menu.Path,
			"icon":     menu.Icon,
			"isActive": true,
			"items":    children,
		})
	}
	return result
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

// errInternalDB shorthand untuk response error koneksi DB
func errInternalDB(ctx *fiber.Ctx) error {
	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"message": "Failed to connect to database",
	})
}
