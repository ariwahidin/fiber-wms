package models

import "time"

type ShopeeWebhookLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	ShopID    int64     `gorm:"column:shop_id"`
	PushCode  int       `gorm:"column:push_code"`
	OrderSN   string    `gorm:"column:order_sn;size:50"`
	Status    string    `gorm:"column:status;size:50"`
	RawData   string    `gorm:"column:raw_data;type:text"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (ShopeeWebhookLog) TableName() string {
	return "shopee_webhook_logs"
}
