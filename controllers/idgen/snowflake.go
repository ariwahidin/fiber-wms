// internal/idgen/snowflake.go
package idgen

import (
	"fiber-app/types"
	"fmt"
	"log"
	"reflect"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
)

var node *snowflake.Node

func Init() {
	var err error
	node, err = snowflake.NewNode(1)
	if err != nil {
		log.Fatalf("Failed to init Snowflake: %v", err)
	}
}

func GenerateID() int64 {
	return node.Generate().Int64()
}

func AutoGenerateSnowflakeID(db *gorm.DB) {
	db.Callback().Create().Before("gorm:before_create").Register("snowflake_auto_id", func(tx *gorm.DB) {
		fmt.Println("Auto-generating Snowflake ID...")
		if tx.Statement.Schema == nil {
			return
		}

		for _, field := range tx.Statement.Schema.Fields {
			if field.Name == "ID" && field.FieldType.Kind() == reflect.Int64 {
				// Skip, bukan SnowflakeID
				continue
			}

			// Cek apakah field tipe-nya types.SnowflakeID
			if field.FieldType.String() == "types.SnowflakeID" {
				val := field.ReflectValueOf(tx.Statement.Context, tx.Statement.ReflectValue)
				if val.IsValid() && val.CanSet() && val.Int() == 0 {
					val.Set(reflect.ValueOf(types.SnowflakeID(GenerateID())))
				}
			}
		}
	})
}
