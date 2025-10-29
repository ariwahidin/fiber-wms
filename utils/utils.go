package utils

import (
	"fiber-app/models"

	"gorm.io/gorm"
)

func InsertLog(db *gorm.DB, log models.IntegrationLog) {
	db.Create(&log)
}
