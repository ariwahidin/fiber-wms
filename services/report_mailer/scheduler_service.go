package services

import (
	"fiber-app/models/report_mailer"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// ─── Scheduler Manager ────────────────────────────────────────────────────────

// type SchedulerManager struct {
// 	cron     *cron.Cron
// 	db       *gorm.DB
// 	entryMap map[uint]cron.EntryID // reportID → cron EntryID
// 	mu       sync.Mutex
// }

type SchedulerManager struct {
	cron     *cron.Cron
	db       *gorm.DB
	queryDB  *gorm.DB              // read-only DB khusus eksekusi query report
	entryMap map[uint]cron.EntryID // reportID → cron EntryID
	mu       sync.Mutex
}

var Manager *SchedulerManager

// Init dipanggil sekali saat app start di main.go
// func InitScheduler(db *gorm.DB) {
// 	Manager = &SchedulerManager{
// 		cron:     cron.New(),
// 		db:       db,
// 		entryMap: make(map[uint]cron.EntryID),
// 	}

// 	if err := Manager.loadAll(); err != nil {
// 		log.Printf("⚠️  Scheduler: gagal load schedules: %v", err)
// 	}

// 	Manager.cron.Start()
// 	log.Println("✅ Scheduler: berjalan")
// }

func InitScheduler(db *gorm.DB, queryDB *gorm.DB) {
	Manager = &SchedulerManager{
		cron:     cron.New(),
		db:       db,
		queryDB:  queryDB,
		entryMap: make(map[uint]cron.EntryID),
	}

	if err := Manager.loadAll(); err != nil {
		log.Printf("⚠️  Scheduler: gagal load schedules: %v", err)
	}

	Manager.cron.Start()
	log.Println("✅ Scheduler: berjalan")
}

// ─── Load Semua Schedule dari DB ──────────────────────────────────────────────

func (s *SchedulerManager) loadAll() error {
	var schedules []report_mailer.ReportSchedule
	if err := s.db.Where("is_active = ?", true).Find(&schedules).Error; err != nil {
		return err
	}

	for _, schedule := range schedules {
		if err := s.register(schedule); err != nil {
			log.Printf("⚠️  Scheduler: gagal register report %d: %v", schedule.ReportID, err)
		}
	}

	log.Printf("✅ Scheduler: %d schedule aktif dimuat", len(schedules))
	return nil
}

// ─── Register / Unregister ────────────────────────────────────────────────────

func (s *SchedulerManager) register(schedule report_mailer.ReportSchedule) error {
	expr, err := buildCronExpression(schedule)
	if err != nil {
		return err
	}

	reportID := schedule.ReportID

	entryID, err := s.cron.AddFunc(expr, func() {
		s.runReport(reportID)
	})
	if err != nil {
		return fmt.Errorf("gagal register cron: %w", err)
	}

	s.mu.Lock()
	s.entryMap[reportID] = entryID
	s.mu.Unlock()

	log.Printf("✅ Scheduler: report %d terdaftar [%s]", reportID, expr)
	return nil
}

func (s *SchedulerManager) unregister(reportID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entryMap[reportID]; ok {
		s.cron.Remove(entryID)
		delete(s.entryMap, reportID)
		log.Printf("🗑️  Scheduler: report %d dihapus dari scheduler", reportID)
	}
}

// ─── Reload (dipanggil dari API saat schedule diubah) ─────────────────────────

// ReloadReport reload schedule untuk satu report.
// Dipanggil setelah user simpan/ubah schedule dari UI.
func (s *SchedulerManager) ReloadReport(reportID uint) {
	// Hapus schedule lama kalau ada
	s.unregister(reportID)

	// Load schedule baru dari DB
	var schedule report_mailer.ReportSchedule
	err := s.db.Where("report_id = ? AND is_active = ?", reportID, true).First(&schedule).Error
	if err != nil {
		// Tidak ada schedule aktif untuk report ini, tidak perlu register
		return
	}

	if err := s.register(schedule); err != nil {
		log.Printf("⚠️  Scheduler: gagal reload report %d: %v", reportID, err)
	}
}

// ReloadAll reload ulang semua schedule dari DB.
// Bisa dipanggil via API endpoint kalau diperlukan.
func (s *SchedulerManager) ReloadAll() {
	s.mu.Lock()
	// Hapus semua entry yang ada
	for reportID, entryID := range s.entryMap {
		s.cron.Remove(entryID)
		delete(s.entryMap, reportID)
	}
	s.mu.Unlock()

	if err := s.loadAll(); err != nil {
		log.Printf("⚠️  Scheduler: gagal reload all: %v", err)
	}
}

// ─── Run Report ───────────────────────────────────────────────────────────────

// func (s *SchedulerManager) runReport(reportID uint) {
// 	log.Printf("🚀 Scheduler: menjalankan report %d...", reportID)

// 	// Load report lengkap dengan relasi
// 	var report report_mailer.Report
// 	err := s.db.
// 		Preload("EmailConfig").
// 		Preload("Recipients").
// 		First(&report, reportID).Error

// 	if err != nil {
// 		log.Printf("❌ Scheduler: report %d tidak ditemukan: %v", reportID, err)
// 		LogSendHistory(s.db, reportID, "scheduler", fmt.Errorf("report tidak ditemukan: %w", err))
// 		return
// 	}

// 	if !report.IsActive {
// 		log.Printf("⏭️  Scheduler: report %d tidak aktif, skip", reportID)
// 		return
// 	}

// 	// Kirim report
// 	sendErr := SendReport(s.db, report)

// 	// Catat history
// 	LogSendHistory(s.db, reportID, "scheduler", sendErr)

// 	// Update last_run_at & next_run_at di schedule
// 	s.updateRunTime(reportID)

// 	if sendErr != nil {
// 		log.Printf("❌ Scheduler: report %d gagal: %v", reportID, sendErr)
// 	} else {
// 		log.Printf("✅ Scheduler: report %d berhasil dikirim", reportID)
// 	}
// }

// runReport menjalankan pengiriman report dengan retry otomatis.
// Dipanggil oleh cron scheduler.
func (s *SchedulerManager) runReport(reportID uint) {
	log.Printf("🚀 Scheduler: menjalankan report %d...", reportID)

	// Load report lengkap
	var report report_mailer.Report
	if err := s.db.
		Preload("EmailConfig").
		Preload("Recipients").
		First(&report, reportID).Error; err != nil {
		log.Printf("❌ Scheduler: report %d tidak ditemukan: %v", reportID, err)
		LogSendHistory(s.db, reportID, "scheduler", fmt.Errorf("report tidak ditemukan: %w", err))
		return
	}

	if !report.IsActive {
		log.Printf("⏭️  Scheduler: report %d tidak aktif, skip", reportID)
		return
	}

	// Load schedule untuk konfigurasi retry
	var schedule report_mailer.ReportSchedule
	if err := s.db.Where("report_id = ?", reportID).First(&schedule).Error; err != nil {
		log.Printf("⚠️  Scheduler: schedule report %d tidak ditemukan, jalankan tanpa retry", reportID)
		// sendErr := SendReport(s.db, report)
		sendErr := SendReport(s.db, s.queryDB, report)
		LogSendHistory(s.db, reportID, "scheduler", sendErr)
		s.updateRunTime(reportID)
		return
	}

	// Reset retry count di awal setiap jadwal baru
	s.db.Model(&schedule).Update("retry_count", 0)

	// Jalankan dengan retry
	s.runWithRetry(report, schedule)

	// Update last_run_at & next_run_at
	s.updateRunTime(reportID)
}

// runWithRetry mencoba kirim report, retry kalau gagal.
func (s *SchedulerManager) runWithRetry(report report_mailer.Report, schedule report_mailer.ReportSchedule) {
	maxRetry := schedule.MaxRetry
	if maxRetry <= 0 {
		maxRetry = 1 // minimal 1 percobaan
	}
	delayMin := schedule.RetryDelayMin
	if delayMin <= 0 {
		delayMin = 5
	}

	var lastErr error

	for attempt := 1; attempt <= maxRetry; attempt++ {
		if attempt > 1 {
			log.Printf("🔄 Scheduler: retry %d/%d untuk report %d (jeda %d menit)...",
				attempt, maxRetry, report.ID, delayMin)

			// Jeda sebelum retry
			time.Sleep(time.Duration(delayMin) * time.Minute)

			// Reload report untuk pastikan data masih fresh
			s.db.Preload("EmailConfig").Preload("Recipients").First(&report, report.ID)
		}

		log.Printf("📤 Scheduler: attempt %d/%d report %d", attempt, maxRetry, report.ID)

		// lastErr = SendReport(s.db, report)
		lastErr = SendReport(s.db, s.queryDB, report)

		if lastErr == nil {
			// Sukses
			log.Printf("✅ Scheduler: report %d berhasil dikirim (attempt %d)", report.ID, attempt)
			LogSendHistory(s.db, report.ID, "scheduler", nil)

			// Reset retry count
			s.db.Model(&report_mailer.ReportSchedule{}).
				Where("report_id = ?", report.ID).
				Update("retry_count", 0)
			return
		}

		// Gagal — catat dan update retry count
		log.Printf("❌ Scheduler: report %d attempt %d gagal: %v", report.ID, attempt, lastErr)

		s.db.Model(&report_mailer.ReportSchedule{}).
			Where("report_id = ?", report.ID).
			Update("retry_count", attempt)
	}

	// Semua attempt habis, catat sebagai gagal final
	finalErr := fmt.Errorf("gagal setelah %d percobaan: %w", maxRetry, lastErr)
	log.Printf("💀 Scheduler: report %d gagal total setelah %d attempt", report.ID, maxRetry)
	LogSendHistory(s.db, report.ID, "scheduler", finalErr)
}

// ─── Update Run Time ──────────────────────────────────────────────────────────

func (s *SchedulerManager) updateRunTime(reportID uint) {
	s.mu.Lock()
	entryID, ok := s.entryMap[reportID]
	s.mu.Unlock()

	now := time.Now()
	updates := map[string]interface{}{
		"last_run_at": now,
	}

	if ok {
		next := s.cron.Entry(entryID).Next
		updates["next_run_at"] = next
	}

	s.db.Model(&report_mailer.ReportSchedule{}).
		Where("report_id = ?", reportID).
		Updates(updates)
}

// ─── Manual Trigger ───────────────────────────────────────────────────────────

// TriggerNow menjalankan report sekarang juga tanpa menunggu schedule.
// Dipanggil dari endpoint "Send Now" di UI.
// func TriggerNow(db *gorm.DB, reportID uint) error {
// func TriggerNow(db *gorm.DB, queryDB *gorm.DB, reportID uint) error {
// 	var report report_mailer.Report
// 	if err := db.
// 		Preload("EmailConfig").
// 		Preload("Recipients").
// 		First(&report, reportID).Error; err != nil {
// 		return fmt.Errorf("report tidak ditemukan: %w", err)
// 	}

// 	// sendErr := SendReport(db, report)
// 	sendErr := SendReport(db, queryDB, report)
// 	LogSendHistory(db, reportID, "manual", sendErr)
// 	return sendErr
// }

// ─── Cron Expression Builder ──────────────────────────────────────────────────

// buildCronExpression mengkonversi setting visual ke cron expression.
// Format: "menit jam * * hari_dalam_minggu" atau "menit jam tanggal * *"
func buildCronExpression(schedule report_mailer.ReportSchedule) (string, error) {
	m := schedule.Minute
	h := schedule.Hour

	switch schedule.Frequency {
	case report_mailer.FrequencyDaily:
		// Setiap hari pada jam H:M
		// cth: jam 08:30 → "30 8 * * *"
		return fmt.Sprintf("%d %d * * *", m, h), nil

	case report_mailer.FrequencyWeekly:
		// Setiap hari X dalam seminggu pada jam H:M
		// cth: Senin jam 08:00 → "0 8 * * 1"
		if schedule.DayOfWeek == nil {
			return "", fmt.Errorf("day_of_week wajib diisi untuk frequency weekly")
		}
		return fmt.Sprintf("%d %d * * %d", m, h, *schedule.DayOfWeek), nil

	case report_mailer.FrequencyMonthly:
		// Setiap tanggal X setiap bulan pada jam H:M
		// cth: tanggal 1 jam 07:00 → "0 7 1 * *"
		if schedule.DayOfMonth == nil {
			return "", fmt.Errorf("day_of_month wajib diisi untuk frequency monthly")
		}
		return fmt.Sprintf("%d %d %d * *", m, h, *schedule.DayOfMonth), nil

	default:
		return "", fmt.Errorf("frequency tidak dikenal: %s", schedule.Frequency)
	}
}
