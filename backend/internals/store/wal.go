package store

import (
	"fmt"
	"os"
	"sync"
)

// WAL struct

type WAL struct {
	mu   sync.Mutex
	file *os.File
	path string
}

// function for opening the file

func OpenWAL(path string) (*WAL, error) {

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		fmt.Print("Error opening the file of WAL")
		return nil, err
	}
	return &WAL{
		file: file,
		path: path,
	}, nil

}

// function for the append one

func (w *WAL) Append(record []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Ensure the record ends with '\n'
	if len(record) == 0 || record[len(record)-1] != '\n' {
		record = append(record, '\n')
	}

	if _, err := w.file.Write(record); err != nil {
		return err
	}
	return w.file.Sync()
}

