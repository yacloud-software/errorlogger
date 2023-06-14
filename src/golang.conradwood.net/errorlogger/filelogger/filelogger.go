package filelogger

import (
	"fmt"
	"golang.conradwood.net/go-easyops/utils"
	"os"
	"path/filepath"
	"sync"
)

type FileLogger struct {
	filename    string
	maxmb       int
	rotate_lock sync.Mutex
	logfile     *os.File
}

func Open(filename string) (*FileLogger, error) {
	var err error
	f := &FileLogger{filename: filename, maxmb: 100}
	dir := filepath.Dir(f.filename)
	err = os.MkdirAll(dir, 0777)
	if err != nil {
		return nil, err
	}
	f.logfile, err = utils.OpenWriteFile(f.filename)
	return f, err
}

func (f *FileLogger) WriteString(s string) error {
	_, err := f.Write([]byte(s))
	return err
}
func (f *FileLogger) Write(b []byte) (int, error) {
	f.rotate_lock.Lock()
	defer f.rotate_lock.Unlock()
	n, err := f.logfile.Write(b)
	if err != nil {
		fmt.Printf("Failed to log \"%s\": %s\n", string(b), err)
	}
	return n, err
}
func (f *FileLogger) rotate() {
	fi, err := os.Stat(f.filename)
	if err != nil {
		fmt.Printf("[rotate] Unable to stat %s: %s\n", f.filename, err)
		return
	}
	// get the size
	size := fi.Size()
	maxmb := (int64(f.maxmb) * 1024 * 1024)
	if size < maxmb {
		return
	}
	fmt.Printf("[rotate] Filesize: %d (max: %d), wait for lock\n", size, maxmb)
	f.rotate_lock.Lock()
	defer f.rotate_lock.Unlock()
	fmt.Printf("[rotate] Filesize: %d (max: %d), got lock\n", size, maxmb)

	// check again once we got the lock
	fi, err = os.Stat(f.filename)
	if err != nil {
		fmt.Printf("[rotate] Unable to stat %s: %s\n", f.filename, err)
		return
	}
	// get the size
	size = fi.Size()
	maxmb = (int64(f.maxmb) * 1024 * 1024)
	if size < maxmb {
		return
	}

	fmt.Printf("[rotate] rotating...\n")
	newFilename := fmt.Sprintf("%s.1", f.filename)
	os.Remove(newFilename)
	err = os.Rename(f.filename, newFilename)
	if err != nil {
		fmt.Printf("[rotate] failed to rename %s to %s: %s\n", f.filename, newFilename, err)
		return
	}
	if f.logfile != nil {
		f.logfile.Close()
		f.logfile = nil
	}
	f.logfile, err = utils.OpenWriteFile(f.filename)
	if err != nil {
		fmt.Printf("[rotate] Failed to open logfile: %s\n", err)
		return
	}

}
