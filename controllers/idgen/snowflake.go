// internal/idgen/snowflake.go
package idgen

import (
	"log"

	"github.com/bwmarrin/snowflake"
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
