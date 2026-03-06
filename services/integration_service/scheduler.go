package integration_service

import (
	"fiber-app/models/integration"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// ─── Scheduler Manager ────────────────────────────────────────────────────────

type IntegrationSchedulerManager struct {
	cron     *cron.Cron
	db       *gorm.DB
	queryDB  *gorm.DB
	entryMap map[uint]cron.EntryID // integrationID → cron EntryID
	mu       sync.Mutex
}

var IntegrationScheduler *IntegrationSchedulerManager

// InitIntegrationScheduler dipanggil sekali saat app start di main.go
func InitIntegrationScheduler(db *gorm.DB, queryDB *gorm.DB) {
	IntegrationScheduler = &IntegrationSchedulerManager{
		cron:     cron.New(),
		db:       db,
		queryDB:  queryDB,
		entryMap: make(map[uint]cron.EntryID),
	}

	if err := IntegrationScheduler.loadAll(); err != nil {
		log.Printf("⚠️  IntegrationScheduler: gagal load schedules: %v", err)
	}

	IntegrationScheduler.cron.Start()
	log.Println("✅ IntegrationScheduler: berjalan")
}

// ─── Load Semua Schedule dari DB ──────────────────────────────────────────────

func (s *IntegrationSchedulerManager) loadAll() error {
	var integrations []integration.Integration
	if err := s.db.
		Preload("Connection").
		Preload("Recipients").
		Where("is_active = ? AND timing = ? AND direction = ?",
			true,
			integration.TimingScheduled,
			integration.DirectionOutbound,
		).
		Find(&integrations).Error; err != nil {
		return err
	}

	for _, intg := range integrations {
		if err := s.register(intg); err != nil {
			log.Printf("⚠️  IntegrationScheduler: gagal register integrasi %d (%s): %v",
				intg.ID, intg.Name, err)
		}
	}

	log.Printf("✅ IntegrationScheduler: %d integrasi terjadwal dimuat", len(integrations))
	return nil
}

// ─── Register / Unregister ────────────────────────────────────────────────────

func (s *IntegrationSchedulerManager) register(intg integration.Integration) error {
	expr, err := buildIntegrationCronExpression(intg)
	if err != nil {
		return fmt.Errorf("gagal build cron expression: %w", err)
	}

	integrationID := intg.ID

	entryID, err := s.cron.AddFunc(expr, func() {
		s.runJob(integrationID)
	})
	if err != nil {
		return fmt.Errorf("gagal register cron: %w", err)
	}

	s.mu.Lock()
	s.entryMap[integrationID] = entryID
	s.mu.Unlock()

	log.Printf("✅ IntegrationScheduler: integrasi %d (%s) terdaftar [%s]",
		intg.ID, intg.Name, expr)
	return nil
}

func (s *IntegrationSchedulerManager) unregister(integrationID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entryMap[integrationID]; ok {
		s.cron.Remove(entryID)
		delete(s.entryMap, integrationID)
		log.Printf("🗑️  IntegrationScheduler: integrasi %d dihapus dari scheduler", integrationID)
	}
}

// ─── Reload ───────────────────────────────────────────────────────────────────

// ReloadIntegration reload schedule untuk satu integrasi.
// Dipanggil dari controller setelah user simpan/ubah integrasi.
func (s *IntegrationSchedulerManager) ReloadIntegration(integrationID uint) {
	s.unregister(integrationID)

	var intg integration.Integration
	err := s.db.
		Preload("Connection").
		Preload("Recipients").
		Where("id = ? AND is_active = ? AND timing = ? AND direction = ?",
			integrationID, true, integration.TimingScheduled, integration.DirectionOutbound,
		).
		First(&intg).Error

	if err != nil {
		// Tidak ada integrasi scheduled aktif — tidak perlu register
		return
	}

	if err := s.register(intg); err != nil {
		log.Printf("⚠️  IntegrationScheduler: gagal reload integrasi %d: %v", integrationID, err)
	}
}

// ReloadAll reload ulang semua schedule dari DB.
func (s *IntegrationSchedulerManager) ReloadAll() {
	s.mu.Lock()
	for id, entryID := range s.entryMap {
		s.cron.Remove(entryID)
		delete(s.entryMap, id)
	}
	s.mu.Unlock()

	if err := s.loadAll(); err != nil {
		log.Printf("⚠️  IntegrationScheduler: gagal reload all: %v", err)
	}
}

// ─── Run Job ──────────────────────────────────────────────────────────────────

func (s *IntegrationSchedulerManager) runJob(integrationID uint) {
	log.Printf("🚀 IntegrationScheduler: menjalankan integrasi %d...", integrationID)

	// Load integrasi lengkap
	var intg integration.Integration
	if err := s.db.
		Preload("Connection").
		Preload("Recipients").
		First(&intg, integrationID).Error; err != nil {
		log.Printf("❌ IntegrationScheduler: integrasi %d tidak ditemukan: %v", integrationID, err)
		return
	}

	if !intg.IsActive {
		log.Printf("⏭️  IntegrationScheduler: integrasi %d tidak aktif, skip", integrationID)
		return
	}

	// Build event data untuk scheduled job
	// Tidak ada event trigger dari WMS — isi dengan data waktu + scheduled flag
	now := time.Now()
	eventData := map[string]interface{}{
		"scheduled":  true,
		"trigger_by": "scheduler",
		"date":       now.Format("2006-01-02"),
		"datetime":   now.Format("2006-01-02 15:04:05"),
		"year":       now.Format("2006"),
		"month":      now.Format("01"),
	}

	// Jalankan integrasi
	sendErr := runIntegration(s.db, s.queryDB, intg, eventData, "scheduler")

	// Log history
	// logIntegrationHistory(s.db, intg, eventData, sendErr, "scheduler")
	logHistory(s.db, intg, intg.EventKey, sendErr, "scheduler", eventData)

	// Kirim notifikasi email
	sendNotification(s.db, intg, eventData, sendErr)

	// Update last_run_at & next_run_at
	s.updateRunTime(integrationID)

	if sendErr != nil {
		log.Printf("❌ IntegrationScheduler: integrasi %d gagal: %v", integrationID, sendErr)
	} else {
		log.Printf("✅ IntegrationScheduler: integrasi %d berhasil", integrationID)
	}
}

// ─── Update Run Time ──────────────────────────────────────────────────────────

func (s *IntegrationSchedulerManager) updateRunTime(integrationID uint) {
	s.mu.Lock()
	entryID, ok := s.entryMap[integrationID]
	s.mu.Unlock()

	now := time.Now()
	updates := map[string]interface{}{
		"last_run_at": now,
	}

	if ok {
		next := s.cron.Entry(entryID).Next
		updates["next_run_at"] = next
	}

	s.db.Model(&integration.Integration{}).
		Where("id = ?", integrationID).
		Updates(updates)
}

// ─── Manual Trigger ───────────────────────────────────────────────────────────

// TriggerIntegrationNow jalankan integrasi sekarang tanpa menunggu schedule.
// Dipanggil dari endpoint "Run Now" di UI.
func TriggerIntegrationNow(db *gorm.DB, queryDB *gorm.DB, integrationID uint) error {
	var intg integration.Integration
	if err := db.
		Preload("Connection").
		Preload("Recipients").
		First(&intg, integrationID).Error; err != nil {
		return fmt.Errorf("integrasi tidak ditemukan: %w", err)
	}

	now := time.Now()
	eventData := map[string]interface{}{
		"scheduled":  false,
		"trigger_by": "manual",
		"date":       now.Format("2006-01-02"),
		"datetime":   now.Format("2006-01-02 15:04:05"),
		"year":       now.Format("2006"),
		"month":      now.Format("01"),
	}

	sendErr := runIntegration(db, queryDB, intg, eventData, "manual")
	// logIntegrationHistory(db, intg, eventData, sendErr, "manual")
	logHistory(db, intg, intg.EventKey, sendErr, "manual", eventData)
	sendNotification(db, intg, eventData, sendErr)

	return sendErr
}

// ─── Cron Expression Builder ──────────────────────────────────────────────────

func buildIntegrationCronExpression(intg integration.Integration) (string, error) {
	m := intg.ScheduleMinute
	h := intg.ScheduleHour

	switch intg.ScheduleFreq {
	case "daily":
		// Setiap hari pada jam H:M → "30 8 * * *"
		return fmt.Sprintf("%d %d * * *", m, h), nil

	case "weekly":
		// Setiap hari X dalam seminggu → "0 8 * * 1"
		if intg.ScheduleDayOfWeek == nil {
			return "", fmt.Errorf("schedule_day_of_week wajib diisi untuk frequency weekly")
		}
		return fmt.Sprintf("%d %d * * %d", m, h, *intg.ScheduleDayOfWeek), nil

	case "monthly":
		// Setiap tanggal X setiap bulan → "0 7 1 * *"
		if intg.ScheduleDayOfMonth == nil {
			return "", fmt.Errorf("schedule_day_of_month wajib diisi untuk frequency monthly")
		}
		return fmt.Sprintf("%d %d %d * *", m, h, *intg.ScheduleDayOfMonth), nil

	default:
		return "", fmt.Errorf("schedule_freq tidak dikenal: '%s' (gunakan: daily/weekly/monthly)",
			intg.ScheduleFreq)
	}
}
