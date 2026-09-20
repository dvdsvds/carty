package backup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// backupName maps an absolute target file path to a flat file name inside
// the backup directory, e.g. "/home/user/.zshrc" -> "_home_user_.zshrc".
func backupName(targetFile string) string {
	return strings.ReplaceAll(targetFile, string(os.PathSeparator), "_")
}

func backupPath(backupDir, targetFile string) string {
	return filepath.Join(backupDir, backupName(targetFile))
}

// EnsureBackup preserves the pre-carty contents of targetFile the first
// time carty ever touches it. Subsequent calls are no-ops so the backup
// always reflects the true original, never an intermediate carty state.
//
// If targetFile does not exist yet, no backup file is written; its absence
// is itself the signal (on Restore) that carty created the file from
// scratch and it should be removed rather than restored.
func EnsureBackup(backupDir, targetFile string) error {
	dst := backupPath(backupDir, targetFile)

	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	data, err := os.ReadFile(targetFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// Restore reverts targetFile to the state it was in before carty ever
// modified it. If no backup exists, targetFile is assumed to have been
// created by carty and is removed.
func Restore(backupDir, targetFile string) error {
	src := backupPath(backupDir, targetFile)

	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			rmErr := os.Remove(targetFile)
			if rmErr != nil && os.IsNotExist(rmErr) {
				return nil
			}
			return rmErr
		}
		return err
	}

	tmp := targetFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, targetFile)
}

// Has reports whether a backup has already been captured for targetFile.
func Has(backupDir, targetFile string) bool {
	_, err := os.Stat(backupPath(backupDir, targetFile))
	return err == nil
}

var ErrNotBackedUp = errors.New("backup: no backup recorded for target file")
