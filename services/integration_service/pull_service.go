package integration_service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	integrationModel "fiber-app/models/integration"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// ─── Main Pull Entry Point ────────────────────────────────────────────────────

// PullAndProcess dijalankan oleh scheduler atau manual trigger.
// Baca file dari source, proses, lalu archive/error.
func PullAndProcess(db *gorm.DB, intg integrationModel.Integration, userID int) ProcessResult {
	conn := intg.Connection
	if conn == nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Connection belum dikonfigurasi"}},
		}
	}

	switch intg.ChannelType {
	case integrationModel.ChannelSFTP:
		return pullFromSFTP(db, intg, *conn, userID)
	case integrationModel.ChannelFTP:
		return pullFromFTP(db, intg, *conn, userID)
	case integrationModel.ChannelFile:
		return pullFromFolder(db, intg, *conn, userID)
	default:
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Channel tidak support pull: " + string(intg.ChannelType)}},
		}
	}
}

// ─── SFTP Pull ────────────────────────────────────────────────────────────────

func pullFromSFTP(db *gorm.DB, intg integrationModel.Integration, conn integrationModel.IntegrationConnection, userID int) ProcessResult {
	port := conn.Port
	if port == 0 {
		port = 22
	}

	sshConfig := &ssh.ClientConfig{
		User:            conn.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(conn.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", conn.Host, port), sshConfig)
	if err != nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Gagal koneksi SFTP", Detail: err.Error()}},
		}
	}
	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Gagal buat SFTP client", Detail: err.Error()}},
		}
	}
	defer sftpClient.Close()

	sourcePath := intg.SourcePath
	if sourcePath == "" {
		sourcePath = conn.RemotePath
	}

	// List file di source path
	fileInfos, err := sftpClient.ReadDir(sourcePath)
	if err != nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Gagal baca direktori SFTP: " + sourcePath, Detail: err.Error()}},
		}
	}

	var overallResult ProcessResult
	for _, fi := range fileInfos {
		if fi.IsDir() {
			continue
		}
		if !isSupportedFormat(fi.Name()) {
			continue
		}

		remoteFilePath := sourcePath + "/" + fi.Name()

		// Baca file
		f, err := sftpClient.Open(remoteFilePath)
		if err != nil {
			overallResult.FailedCount++
			overallResult.Errors = append(overallResult.Errors, ProcessError{
				Row:     0,
				Message: "Gagal buka file: " + fi.Name(),
				Detail:  err.Error(),
			})
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			overallResult.FailedCount++
			overallResult.Errors = append(overallResult.Errors, ProcessError{
				Row:     0,
				Message: "Gagal baca file: " + fi.Name(),
				Detail:  err.Error(),
			})
			continue
		}

		// Proses file
		result := processFileData(db, intg, data, fi.Name(), userID)
		mergeResult(&overallResult, result)

		// Move file ke archive atau error path
		destPath := buildDestPath(intg.ArchivePath, fi.Name(), result.FailedCount > 0)
		if result.FailedCount > 0 && intg.ErrorPath != "" {
			destPath = buildDestPath(intg.ErrorPath, fi.Name(), true)
		}

		// Pastikan folder tujuan ada di SFTP
		destDir := filepath.Dir(destPath)
		sftpClient.MkdirAll(destDir)

		// Rename (move)
		if err := sftpClient.Rename(remoteFilePath, destPath); err != nil {
			// Kalau rename gagal, cukup log — jangan block proses
			overallResult.Errors = append(overallResult.Errors, ProcessError{
				Row:     0,
				Message: "Warning: gagal move file ke archive: " + fi.Name(),
				Detail:  err.Error(),
			})
		}
	}

	return overallResult
}

// ─── FTP Pull ─────────────────────────────────────────────────────────────────

func pullFromFTP(db *gorm.DB, intg integrationModel.Integration, conn integrationModel.IntegrationConnection, userID int) ProcessResult {
	port := conn.Port
	if port == 0 {
		port = 21
	}

	c, err := ftp.Dial(fmt.Sprintf("%s:%d", conn.Host, port))
	if err != nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Gagal koneksi FTP", Detail: err.Error()}},
		}
	}
	defer c.Quit()

	if err := c.Login(conn.Username, conn.Password); err != nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Gagal login FTP", Detail: err.Error()}},
		}
	}

	sourcePath := intg.SourcePath
	if sourcePath == "" {
		sourcePath = conn.RemotePath
	}

	entries, err := c.List(sourcePath)
	if err != nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Gagal list direktori FTP", Detail: err.Error()}},
		}
	}

	var overallResult ProcessResult
	for _, entry := range entries {
		if entry.Type != ftp.EntryTypeFile {
			continue
		}
		if !isSupportedFormat(entry.Name) {
			continue
		}

		remoteFilePath := sourcePath + "/" + entry.Name

		resp, err := c.Retr(remoteFilePath)
		if err != nil {
			overallResult.FailedCount++
			overallResult.Errors = append(overallResult.Errors, ProcessError{
				Row:     0,
				Message: "Gagal download file FTP: " + entry.Name,
				Detail:  err.Error(),
			})
			continue
		}
		data, err := io.ReadAll(resp)
		resp.Close()
		if err != nil {
			overallResult.FailedCount++
			overallResult.Errors = append(overallResult.Errors, ProcessError{
				Row:     0,
				Message: "Gagal baca file FTP: " + entry.Name,
				Detail:  err.Error(),
			})
			continue
		}

		result := processFileData(db, intg, data, entry.Name, userID)
		mergeResult(&overallResult, result)

		// Move ke archive / error
		if result.FailedCount == 0 && intg.ArchivePath != "" {
			archiveDest := intg.ArchivePath + "/" + entry.Name
			c.Rename(remoteFilePath, archiveDest)
		} else if result.FailedCount > 0 && intg.ErrorPath != "" {
			errorDest := intg.ErrorPath + "/" + entry.Name
			c.Rename(remoteFilePath, errorDest)
		}
	}

	return overallResult
}

// ─── Folder Pull ──────────────────────────────────────────────────────────────

func pullFromFolder(db *gorm.DB, intg integrationModel.Integration, conn integrationModel.IntegrationConnection, userID int) ProcessResult {
	sourcePath := intg.SourcePath
	if sourcePath == "" {
		sourcePath = conn.OutputPath
	}
	if sourcePath == "" {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "source_path belum dikonfigurasi"}},
		}
	}

	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Gagal baca folder: " + sourcePath, Detail: err.Error()}},
		}
	}

	var overallResult ProcessResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isSupportedFormat(entry.Name()) {
			continue
		}

		fullPath := filepath.Join(sourcePath, entry.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			overallResult.FailedCount++
			overallResult.Errors = append(overallResult.Errors, ProcessError{
				Row:     0,
				Message: "Gagal baca file: " + entry.Name(),
				Detail:  err.Error(),
			})
			continue
		}

		result := processFileData(db, intg, data, entry.Name(), userID)
		mergeResult(&overallResult, result)

		// Move ke archive atau error path
		isError := result.FailedCount > 0
		destDir := intg.ArchivePath
		if isError && intg.ErrorPath != "" {
			destDir = intg.ErrorPath
		}

		if destDir != "" {
			os.MkdirAll(destDir, 0755)
			destFile := filepath.Join(destDir, timestampedFilename(entry.Name()))
			os.Rename(fullPath, destFile)
		}
	}

	return overallResult
}

// ─── Process File Data ────────────────────────────────────────────────────────

func processFileData(db *gorm.DB, intg integrationModel.Integration, data []byte, filename string, userID int) ProcessResult {
	// Parse file
	rows, err := ReadFile(data, string(intg.FileFormat), filename)
	if err != nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Gagal parse file: " + filename, Detail: err.Error()}},
		}
	}
	if len(rows) == 0 {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "File kosong atau tidak ada data: " + filename}},
		}
	}

	// Proses rows → create outbound
	return ProcessInboundRows(db, intg, rows, userID)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func isSupportedFormat(filename string) bool {
	lower := strings.ToLower(filename)
	for _, ext := range []string{".csv", ".xlsx", ".xls", ".json", ".xml", ".txt"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func buildDestPath(basePath, filename string, isError bool) string {
	if basePath == "" {
		return ""
	}
	return basePath + "/" + timestampedFilename(filename)
}

func timestampedFilename(filename string) string {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	return fmt.Sprintf("%s_%s%s", name, time.Now().Format("20060102_150405"), ext)
}

func mergeResult(target *ProcessResult, src ProcessResult) {
	target.TotalRows += src.TotalRows
	target.SuccessCount += src.SuccessCount
	target.FailedCount += src.FailedCount
	target.Errors = append(target.Errors, src.Errors...)
	target.OutboundNos = append(target.OutboundNos, src.OutboundNos...)
}
