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

	fmt.Println("Login attempt for email:", input.Email)
	fmt.Println("User-Agent:", ctx.Get("User-Agent"))

	db, err := database.GetDBConnection(config.DBUnit)

	if err != nil {
		fmt.Println("Database connection error:", err)
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

	// Expire stale sessions first. A JWT may expire while its DB session
	// remains marked active, so is_active alone is not sufficient.
	if err := db.Model(&models.UserSession{}).
		Where("user_id = ? AND is_active = ? AND expires_at <= ?", mUser.ID, true, now).
		Update("is_active", false).Error; err != nil {
		fmt.Println("Failed to expire stale sessions:", err)
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to validate existing sessions",
		})
	}

	// Only a session that is still active AND not expired should trigger conflict.
	var active models.UserSession
	err = db.Where(
		"user_id = ? AND is_active = ? AND expires_at > ?",
		mUser.ID, true, now,
	).Order("last_activity_at DESC").First(&active).Error

	if err == nil {
		conflict := models.LoginConflict{
			ID:        uuid.NewString(),
			UserID:    uint64(mUser.ID),
			ExpiresAt: now.Add(5 * time.Minute),
		}
		if createErr := db.Create(&conflict).Error; createErr != nil {
			fmt.Println("Failed to create login conflict:", createErr)
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create login conflict",
			})
		}

		message := fmt.Sprintf("User already logged in on another device: %s, last activity at: %s",
			active.DeviceID, active.LastActivityAt.Format("2006-01-02 15:04:05"))
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"device":  device,
			"ip":      ip,
			"ua":      ua,
			"cid":     conflict.ID,
			"message": message,
			"error":   message,
		})
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		fmt.Println("Failed to check active sessions:", err)
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to check active sessions",
		})
	}

	// No valid session exists; create a fresh session.
	if err := db.Create(&models.UserSession{
		UserID:         uint64(mUser.ID),
		SessionID:      sessionID,
		DeviceID:       device,
		IPAddress:      ip,
		UserAgent:      ua,
		IsActive:       true,
		LastActivityAt: now,
		ExpiresAt:      now.Add(24 * time.Hour),
	}).Error; err != nil {
		fmt.Println("Failed to create user session:", err)
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create user session",
		})
	}

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
	if !conflict.ExpiresAt.After(time.Now()) {
		return ctx.Status(fiber.StatusGone).JSON(fiber.Map{
			"success": false,
			"message": "Login conflict has expired. Please login again.",
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
	if err := db.Model(&models.UserSession{}).
		Where("user_id = ? AND is_active = ?", mUser.ID, true).
		Update("is_active", false).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to invalidate previous sessions",
		})
	}

	ip, ua, _, _, device := getClientInfo(ctx)
	now := time.Now()
	sessionID := uuid.New().String()

	if err := db.Create(&models.UserSession{
		UserID:         uint64(mUser.ID),
		SessionID:      sessionID,
		IPAddress:      ip,
		UserAgent:      ua,
		IsActive:       true,
		DeviceID:       device,
		LastActivityAt: now,
		ExpiresAt:      now.Add(24 * time.Hour),
	}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create new session",
		})
	}

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

	now := time.Now()
	if err := db.Model(&models.UserSession{}).
		Where("user_id = ? AND is_active = ? AND expires_at <= ?", conflict.UserID, true, now).
		Update("is_active", false).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "failed to clean expired sessions",
		})
	}

	var sessions []models.UserSession
	if err := db.Where("user_id = ? AND is_active = ? AND expires_at > ?", conflict.UserID, true, now).
		Order("last_activity_at DESC").Find(&sessions).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "failed to fetch sessions",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":    true,
		"message":    "Session active",
		"expires_at": conflict.ExpiresAt,
		"data":       sessions,
	})
}

func RefreshToken(ctx *fiber.Ctx) error {
	tokenString := ctx.Cookies("refresh_token")
	if tokenString == "" {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Refresh token not found",
		})
	}

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Refresh token is invalid or expired",
		})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Invalid refresh token claims"})
	}

	// Refresh tokens issued before session_id was added must re-login once.
	sessionID, ok := claims["session_id"].(string)
	if !ok || sessionID == "" {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Refresh token has no session. Please login again."})
	}

	db, err := database.GetDBConnection(config.DBUnit)
	if err != nil {
		return errInternalDB(ctx)
	}

	var session models.UserSession
	if err := db.Where("session_id = ? AND is_active = ? AND expires_at > ?", sessionID, true, time.Now()).First(&session).Error; err != nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Session is no longer active. Please login again."})
	}

	var mUser models.User
	if err := db.First(&mUser, session.UserID).Error; err != nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "User not found"})
	}

	permissionStrings, err := fetchPermissionStrings(db, mUser.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to refresh permissions"})
	}
	var userWithRoles models.User
	if err := db.Preload("Roles").First(&userWithRoles, mUser.ID).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to refresh roles"})
	}
	roleNames := make([]string, 0, len(userWithRoles.Roles))
	for _, role := range userWithRoles.Roles {
		roleNames = append(roleNames, role.Name)
	}

	accessToken, refreshToken, err := generateTokenPair(mUser.ID, sessionID, permissionStrings, roleNames)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to generate tokens"})
	}
	ctx.Cookie(config.GetTokenCookie(refreshToken))

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":      true,
		"message":      "Token refreshed successfully",
		"x_token":      accessToken,
		"access_token": accessToken,
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
	now := time.Now()
	accessExpiry := now.Add(15 * time.Minute).Unix()
	refreshExpiry := now.Add(7 * 24 * time.Hour).Unix()
	secret := []byte(config.JWTSecret)

	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":     userID,
		"session_id":  sessionID,
		"permissions": permissions, // ["supplier:read", "inbound:create", ...]
		"roles":       roles,       // ["SUPERADMIN"] — untuk bypass di middleware
		"exp":         accessExpiry,
		"unit":        config.DBUnit,
		"jti":         uuid.NewString(),
	})

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    userID,
		"session_id": sessionID,
		"unit":       config.DBUnit,
		"exp":        refreshExpiry,
		"jti":        uuid.NewString(),
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
	case strings.Contains(uaLower, "edg/"):
		browser = "Edge"
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
