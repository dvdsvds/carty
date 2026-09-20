package apply

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dvdsvds/carty/internal/backup"
)

// ErrCorruptMarker is returned when a start marker is found without its
// matching end marker, meaning the user (or something else) has partially
// edited or deleted carty's managed section by hand. Apply refuses to
// guess in that case; the caller decides whether to restore from backup.
var ErrCorruptMarker = errors.New("apply: marker section is corrupt")

func startMarker(commentPrefix, categoryID string) string {
	return fmt.Sprintf("%s >>> carty: %s >>>", commentPrefix, categoryID)
}

func endMarker(commentPrefix, categoryID string) string {
	return fmt.Sprintf("%s <<< carty: %s <<<", commentPrefix, categoryID)
}

func replaceSection(content, commentPrefix, categoryID, newBody string) (string, error) {
	start := startMarker(commentPrefix, categoryID)
	end := endMarker(commentPrefix, categoryID)

	startIdx := strings.Index(content, start)
	if startIdx == -1 {
		if content == "" {
			return start + "\n" + newBody + "\n" + end + "\n", nil
		}
		return content + "\n" + start + "\n" + newBody + "\n" + end + "\n", nil
	}

	relEndIdx := strings.Index(content[startIdx:], end)
	if relEndIdx == -1 {
		return "", ErrCorruptMarker
	}
	endIdx := startIdx + relEndIdx

	before := content[:startIdx]
	after := content[endIdx+len(end):]

	return before + start + "\n" + newBody + "\n" + end + after, nil
}

// removeSection deletes a previously applied section entirely (used when a
// category is no longer selected in the cart). If the section isn't
// present, content is returned unchanged.
func removeSection(content, commentPrefix, categoryID string) (string, error) {
	start := startMarker(commentPrefix, categoryID)
	end := endMarker(commentPrefix, categoryID)

	startIdx := strings.Index(content, start)
	if startIdx == -1 {
		return content, nil
	}

	relEndIdx := strings.Index(content[startIdx:], end)
	if relEndIdx == -1 {
		return "", ErrCorruptMarker
	}
	endIdx := startIdx + relEndIdx + len(end)

	before := strings.TrimRight(content[:startIdx], "\n")
	after := content[endIdx:]
	if before == "" {
		return strings.TrimPrefix(after, "\n"), nil
	}
	return before + after, nil
}

func writeAtomic(path, content string) error {
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

func readOrEmpty(targetFile string) (string, error) {
	data, err := os.ReadFile(targetFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// Apply backs up targetFile on first touch, then writes/replaces the
// section for categoryID with newBody.
func Apply(backupDir, targetFile, commentPrefix, categoryID, newBody string) error {
	if err := backup.EnsureBackup(backupDir, targetFile); err != nil {
		return err
	}

	content, err := readOrEmpty(targetFile)
	if err != nil {
		return err
	}

	newContent, err := replaceSection(content, commentPrefix, categoryID, newBody)
	if err != nil {
		return err
	}

	return writeAtomic(targetFile, newContent)
}

// Remove deletes the section for categoryID from targetFile, used when a
// previously-applied category is left out of the current cart.
func Remove(targetFile, commentPrefix, categoryID string) error {
	content, err := readOrEmpty(targetFile)
	if err != nil {
		return err
	}

	newContent, err := removeSection(content, commentPrefix, categoryID)
	if err != nil {
		return err
	}
	if newContent == content {
		return nil
	}

	return writeAtomic(targetFile, newContent)
}

// RestoreFromBackup reverts targetFile to its pre-carty state, used after
// the user consents to recovering from a corrupt marker section.
func RestoreFromBackup(backupDir, targetFile string) error {
	return backup.Restore(backupDir, targetFile)
}
