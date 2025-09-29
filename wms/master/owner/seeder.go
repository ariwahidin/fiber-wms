package owner

import (
	"gorm.io/gorm"
)

func SeedOwner(db *gorm.DB) {
	owners := []Owner{
		{Code: "YMID", Name: "YMID", Description: "Yamaha Music Indonesia"},
	}

	for _, o := range owners {
		var existing Owner
		if err := db.Where("code = ?", o.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// o.ID = types.SnowflakeID(idgen.GenerateID())
				db.Create(&o)
			}
		}
	}
}
