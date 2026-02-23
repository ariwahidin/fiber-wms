package integration_service

import (
	"bytes"
	"fiber-app/models/integration"
	"fmt"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func sendViaSFTP(conn integration.IntegrationConnection, file *GeneratedFile) error {
	if conn.Host == "" {
		return fmt.Errorf("SFTP host tidak dikonfigurasi")
	}

	port := conn.Port
	if port == 0 {
		port = 22
	}

	sshConfig := &ssh.ClientConfig{
		User: conn.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(conn.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // production: ganti dengan known_hosts
	}

	addr := fmt.Sprintf("%s:%d", conn.Host, port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("gagal koneksi SSH ke %s: %w", addr, err)
	}
	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("gagal buat SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Pastikan remote path ada
	remotePath := conn.RemotePath
	if remotePath == "" {
		remotePath = "/"
	}

	remoteFile := fmt.Sprintf("%s/%s", remotePath, file.Name)

	// Buat/overwrite file di remote
	dstFile, err := sftpClient.Create(remoteFile)
	if err != nil {
		return fmt.Errorf("gagal buat file di SFTP %s: %w", remoteFile, err)
	}
	defer dstFile.Close()

	if _, err := dstFile.ReadFrom(bytes.NewReader(file.Data)); err != nil {
		return fmt.Errorf("gagal upload file ke SFTP: %w", err)
	}

	return nil
}
