package integration_service

import (
	"bytes"
	"fiber-app/models/integration"
	"fmt"

	"github.com/jlaffaye/ftp"
)

func sendViaFTP(conn integration.IntegrationConnection, file *GeneratedFile) error {
	if conn.Host == "" {
		return fmt.Errorf("FTP host tidak dikonfigurasi")
	}

	port := conn.Port
	if port == 0 {
		port = 21
	}

	addr := fmt.Sprintf("%s:%d", conn.Host, port)
	c, err := ftp.Dial(addr)
	if err != nil {
		return fmt.Errorf("gagal koneksi FTP ke %s: %w", addr, err)
	}
	defer c.Quit()

	if err := c.Login(conn.Username, conn.Password); err != nil {
		return fmt.Errorf("gagal login FTP: %w", err)
	}

	remotePath := conn.RemotePath
	if remotePath != "" {
		if err := c.ChangeDir(remotePath); err != nil {
			return fmt.Errorf("gagal pindah ke direktori %s: %w", remotePath, err)
		}
	}

	if err := c.Stor(file.Name, bytes.NewReader(file.Data)); err != nil {
		return fmt.Errorf("gagal upload file ke FTP: %w", err)
	}

	return nil
}
