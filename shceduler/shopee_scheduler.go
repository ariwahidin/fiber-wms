package scheduler

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	outbound_controller "fiber-app/controllers/outbound_controller"

	"gorm.io/gorm"
)

// ============================================================
// SHOPEE SCHEDULER
// Jalankan di main.go setelah setup DB:
//   go scheduler.StartShopeeScheduler(db, queryDB)
// ============================================================

func StartShopeeScheduler(db *gorm.DB, queryDB *gorm.DB) {
	ctrl := outbound_controller.NewShopeeSyncController(db, queryDB)

	// Interval dari env, default 10 menit
	intervalMinutes := 10
	if v := os.Getenv("SHOPEE_SYNC_INTERVAL_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			intervalMinutes = n
		}
	}

	// System userID untuk cron (bisa buat dedicated user di DB)
	cronUserID := 1
	if v := os.Getenv("SHOPEE_CRON_USER_ID"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cronUserID = n
		}
	}

	// Token refresh interval — default 3 jam (token expire 4 jam)
	tokenRefreshHours := 3
	if v := os.Getenv("SHOPEE_TOKEN_REFRESH_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			tokenRefreshHours = n
		}
	}

	log.Printf("[Shopee Scheduler] Started — sync every %d minutes, token refresh every %d hours",
		intervalMinutes, tokenRefreshHours)

	// Token refresh ticker
	tokenTicker := time.NewTicker(time.Duration(tokenRefreshHours) * time.Hour)

	// Sync ticker
	syncTicker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)

	// Jalankan sync pertama kali langsung saat startup
	go func() {
		log.Println("[Shopee Scheduler] Running initial sync on startup...")
		runSync(ctrl, cronUserID)
	}()

	go func() {
		for {
			select {

			case <-tokenTicker.C:
				log.Println("[Shopee Scheduler] Refreshing access token...")
				// if err := ctrl.RefreshToken(); err != nil {
				if err := ctrl.RefreshTokenFromDB(); err != nil {
					log.Printf("[Shopee Scheduler] Token refresh failed: %v", err)
				} else {
					log.Println("[Shopee Scheduler] Token refreshed successfully")
				}

			case <-syncTicker.C:
				log.Printf("[Shopee Scheduler] Running scheduled sync at %s", time.Now().Format("2006-01-02 15:04:05"))
				runSync(ctrl, cronUserID)
			}
		}
	}()
}

func runSync(ctrl *outbound_controller.ShopeeSyncController, userID int) {
	result := ctrl.RunSync(userID)
	if result.Success {
		if result.Synced > 0 {
			log.Printf("[Shopee Scheduler] ✅ Synced %d new orders: %v", result.Synced, result.OutboundNos)
		} else {
			log.Printf("[Shopee Scheduler] ℹ️  No new orders (skipped: %d)", result.Skipped)
		}
		if result.Failed > 0 {
			log.Printf("[Shopee Scheduler] ⚠️  Failed %d orders: %v", result.Failed, result.Errors)
		}
	} else {
		log.Printf("[Shopee Scheduler] ❌ Sync failed: %s", result.Message)
	}
	fmt.Printf("[Shopee Scheduler] Summary — Total: %d | Synced: %d | Skipped: %d | Failed: %d\n",
		result.TotalOrders, result.Synced, result.Skipped, result.Failed)
}
