package server

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go-stock-prediction/pkg/logger"
)

// BackupInfo holds metadata about a single backup file.
type BackupInfo struct {
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	SizeHuman string    `json:"size_human"`
}

// getBackupDir returns the directory where backup files are stored.
// Reads the BACKUP_DIR environment variable, falling back to "/backups".
func getBackupDir() string {
	dir := os.Getenv("BACKUP_DIR")
	if dir == "" {
		return "/backups"
	}
	return dir
}

// formatSizeHuman converts a byte count to a human-readable string such as "1.2 MB".
func formatSizeHuman(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// validateBackupFilename checks that the filename is safe to use:
// no path separators, no "..", must start with "backup_" and end with ".sql.gz".
func validateBackupFilename(filename string) bool {
	if strings.Contains(filename, "..") {
		return false
	}
	if strings.Contains(filename, "/") || strings.Contains(filename, string(os.PathSeparator)) {
		return false
	}
	if !strings.HasPrefix(filename, "backup_") {
		return false
	}
	if !strings.HasSuffix(filename, ".sql.gz") {
		return false
	}
	return true
}

// ListBackupsHandler godoc
//
//	@Summary      List database backups
//	@Description  Returns a list of all available database backup files, sorted newest first.
//	@Tags         Backup
//	@Produce      json
//	@Success      200  {array}   BackupInfo
//	@Failure      401  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/backups [get]
func ListBackupsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	backupDir := getBackupDir()

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			ResponseSuccess(w, http.StatusOK, []BackupInfo{})
			return
		}
		logger.Logger.Errorf("Failed to read backup directory %s: %v", backupDir, err)
		ResponseError(w, http.StatusInternalServerError, "failed to read backup directory")
		return
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "backup_") || !strings.HasSuffix(name, ".sql.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			logger.Logger.Errorf("Failed to stat backup file %s: %v", name, err)
			continue
		}
		backups = append(backups, BackupInfo{
			Filename:  name,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
			SizeHuman: formatSizeHuman(info.Size()),
		})
	}

	// Sort newest first by modification time.
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	if backups == nil {
		backups = []BackupInfo{}
	}

	ResponseSuccess(w, http.StatusOK, backups)
}

// TriggerBackupHandler godoc
//
//	@Summary      Trigger a database backup
//	@Description  Runs mysqldump, compresses the output with gzip, and saves it to the backup directory. Keeps only the 10 most recent backups. Requires admin role.
//	@Tags         Backup
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Failure      401  {object}  ResponseFailure
//	@Failure      403  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/backup [post]
func TriggerBackupHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	// Collect DB connection params from environment.
	host := os.Getenv("MYSQL_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("MYSQL_PORT")
	if port == "" {
		port = "3306"
	}
	user := os.Getenv("MYSQL_USER")
	if user == "" {
		user = "root"
	}
	password := os.Getenv("MYSQL_PASSWORD")
	dbName := os.Getenv("MYSQL_DB_NAME")
	if dbName == "" {
		dbName = "go_stock_prediction"
	}

	backupDir := getBackupDir()
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		logger.Logger.Errorf("Failed to create backup directory %s: %v", backupDir, err)
		ResponseError(w, http.StatusInternalServerError, "failed to create backup directory")
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("backup_%s.sql.gz", timestamp)
	filePath := filepath.Join(backupDir, filename)

	outFile, err := os.Create(filePath)
	if err != nil {
		logger.Logger.Errorf("Failed to create backup file %s: %v", filePath, err)
		ResponseError(w, http.StatusInternalServerError, "failed to create backup file")
		return
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	// Build mysqldump command.
	args := []string{
		fmt.Sprintf("--host=%s", host),
		fmt.Sprintf("--port=%s", port),
		fmt.Sprintf("--user=%s", user),
	}
	if password != "" {
		args = append(args, fmt.Sprintf("--password=%s", password))
	}
	args = append(args,
		"--single-transaction",
		"--routines",
		"--triggers",
		"--add-drop-table",
		dbName,
	)

	cmd := exec.CommandContext(r.Context(), "mysqldump", args...)
	cmd.Stdout = gzWriter

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	logger.Logger.Infof("Starting database backup to %s", filename)
	if err := cmd.Run(); err != nil {
		// Remove the incomplete file.
		outFile.Close()
		os.Remove(filePath)
		logger.Logger.Errorf("mysqldump failed: %v — stderr: %s", err, stderrBuf.String())
		ResponseError(w, http.StatusInternalServerError, fmt.Sprintf("backup failed: %v", err))
		return
	}

	// Flush gzip writer before stat.
	if err := gzWriter.Close(); err != nil {
		logger.Logger.Errorf("Failed to finalize gzip for %s: %v", filename, err)
		ResponseError(w, http.StatusInternalServerError, "failed to finalize backup file")
		return
	}

	info, err := outFile.Stat()
	if err != nil {
		logger.Logger.Errorf("Failed to stat backup file %s: %v", filePath, err)
		ResponseError(w, http.StatusInternalServerError, "backup created but could not read file info")
		return
	}

	logger.Logger.Infof("Backup completed: %s (%s)", filename, formatSizeHuman(info.Size()))

	// Cleanup: keep only the 10 most recent backup files.
	cleanupOldBackups(backupDir, 10)

	ResponseSuccess(w, http.StatusOK, map[string]interface{}{
		"filename": filename,
		"size":     info.Size(),
		"message":  fmt.Sprintf("backup completed successfully (%s)", formatSizeHuman(info.Size())),
	})
}

// cleanupOldBackups removes old backup files so that only `keep` most recent remain.
func cleanupOldBackups(backupDir string, keep int) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		logger.Logger.Errorf("cleanupOldBackups: failed to read dir: %v", err)
		return
	}

	type fileEntry struct {
		name    string
		modTime time.Time
	}

	var files []fileEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "backup_") || !strings.HasSuffix(name, ".sql.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{name: name, modTime: info.ModTime()})
	}

	// Sort newest first.
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	// Remove everything beyond the `keep` limit.
	for i := keep; i < len(files); i++ {
		path := filepath.Join(backupDir, files[i].name)
		if err := os.Remove(path); err != nil {
			logger.Logger.Errorf("cleanupOldBackups: failed to remove %s: %v", path, err)
		} else {
			logger.Logger.Infof("cleanupOldBackups: removed old backup %s", files[i].name)
		}
	}
}

// DownloadBackupHandler godoc
//
//	@Summary      Download a backup file
//	@Description  Streams a backup file as an application/gzip download.
//	@Tags         Backup
//	@Produce      application/gzip
//	@Param        filename  path  string  true  "Backup filename (e.g. backup_20260605_120000.sql.gz)"
//	@Success      200
//	@Failure      400  {object}  ResponseFailure
//	@Failure      401  {object}  ResponseFailure
//	@Failure      404  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/backups/{filename} [get]
func DownloadBackupHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	filename := r.PathValue("filename")
	if !validateBackupFilename(filename) {
		ResponseError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	filePath := filepath.Join(getBackupDir(), filename)

	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			ResponseError(w, http.StatusNotFound, "backup file not found")
			return
		}
		logger.Logger.Errorf("Failed to open backup file %s: %v", filePath, err)
		ResponseError(w, http.StatusInternalServerError, "failed to open backup file")
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		logger.Logger.Errorf("Failed to stat backup file %s: %v", filePath, err)
		ResponseError(w, http.StatusInternalServerError, "failed to read backup file info")
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, f); err != nil {
		logger.Logger.Errorf("Failed to stream backup file %s: %v", filename, err)
	}
}

// DeleteBackupHandler godoc
//
//	@Summary      Delete a backup file
//	@Description  Deletes a backup file from the backup directory. Requires admin role.
//	@Tags         Backup
//	@Param        filename  path  string  true  "Backup filename (e.g. backup_20260605_120000.sql.gz)"
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Failure      400  {object}  ResponseFailure
//	@Failure      401  {object}  ResponseFailure
//	@Failure      403  {object}  ResponseFailure
//	@Failure      404  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/backups/{filename} [delete]
func DeleteBackupHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	filename := r.PathValue("filename")
	if !validateBackupFilename(filename) {
		ResponseError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	filePath := filepath.Join(getBackupDir(), filename)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			ResponseError(w, http.StatusNotFound, "backup file not found")
			return
		}
		logger.Logger.Errorf("Failed to delete backup file %s: %v", filePath, err)
		ResponseError(w, http.StatusInternalServerError, "failed to delete backup file")
		return
	}

	logger.Logger.Infof("Deleted backup file: %s", filename)
	ResponseSuccess(w, http.StatusOK, map[string]interface{}{
		"message":  fmt.Sprintf("backup file %s deleted successfully", filename),
		"filename": filename,
	})
}
