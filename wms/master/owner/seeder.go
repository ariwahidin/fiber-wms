package owner

import (
	"fiber-app/controllers/idgen"
	"fiber-app/types"

	"gorm.io/gorm"
)

func SeedOwner(db *gorm.DB) {
	owners := []Owner{
		{Code: "OWNER1", Name: "Owner 1", Description: "Owner 1 description"},
		{Code: "OWNER2", Name: "Owner 2", Description: "Owner 2 description"},
	}

	for _, o := range owners {
		var existing Owner
		if err := db.Where("code = ?", o.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				o.ID = types.SnowflakeID(idgen.GenerateID())
				db.Create(&o)
			}
		}
	}
}
