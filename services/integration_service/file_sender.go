package integration_service

import (
	"fiber-app/models/integration"
	"fmt"
	"os"
	"path/filepath"
)

func sendViaFile(conn integration.IntegrationConnection, file *GeneratedFile) error {
	outputPath := conn.OutputPath
	if outputPath == "" {
		outputPath = "./output/integration"
	}

	// Buat direktori kalau belum ada
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("gagal buat direktori %s: %w", outputPath, err)
	}

	fullPath := filepath.Join(outputPath, file.Name)
	if err := os.WriteFile(fullPath, file.Data, 0644); err != nil {
		return fmt.Errorf("gagal simpan file ke %s: %w", fullPath, err)
	}

	return nil
}
