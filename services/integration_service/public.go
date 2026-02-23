package integration_service

import (
	"fiber-app/models/integration"

	"gorm.io/gorm"
)

// RunIntegrationPublic adalah wrapper public untuk dipanggil dari controller (TestRun)
func RunIntegrationPublic(db *gorm.DB, queryDB *gorm.DB, intg integration.Integration, eventData map[string]interface{}, triggeredBy string) error {
	return runIntegration(db, queryDB, intg, eventData, triggeredBy)
}

// LogHistoryPublic adalah wrapper public untuk dipanggil dari controller
// func LogHistoryPublic(db *gorm.DB, intg integration.Integration, eventKey string, sendErr error, triggeredBy string) {
// 	logHistory(db, intg, eventKey, sendErr, triggeredBy)
// }

func LogHistoryPublic(db *gorm.DB, intg integration.Integration, eventKey string, sendErr error, triggeredBy string, eventData map[string]interface{}) {
	logHistory(db, intg, eventKey, sendErr, triggeredBy, eventData)
}

// SendNotificationPublic adalah wrapper public untuk sendNotification
func SendNotificationPublic(db *gorm.DB, intg integration.Integration, data map[string]interface{}, sendErr error) {
	sendNotification(db, intg, data, sendErr)
}
