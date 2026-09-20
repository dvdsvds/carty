package lock

import (
	"os"
	"strconv"
	"syscall"
)

func isAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func Acquire(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if !os.IsExist(err) {
			return nil, err
		}

		data, readErr := os.ReadFile(path)
		if readErr == nil {
			pid, convErr := strconv.Atoi(string(data))
			if convErr == nil && isAlive(pid) {
				return nil, err
			}
		}

		os.Remove(path)
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
	}

	pid := os.Getpid()
	f.WriteString(strconv.Itoa(pid))

	return f, nil
}

func Release(path string) error {
	return os.Remove(path)
}
