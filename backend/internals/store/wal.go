package store

import (
	"bufio"
	"fmt"
	"io"
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


// function for the Replay

func (w *WAL) Replay(fn func(line []byte) error) (lastGoodOffset int64, err error){
	// aquire lock
	w.mu.Lock()
	defer w.mu.Unlock()

	// seek to the start

	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return 0, err;
	}
	// initialize the reader

	reader := bufio.NewReader(w.file);
	var offset int64 = 0;

	for {
		// read line
		line, err := reader.ReadBytes('\n');

		// Case: partial record at EOF (bytes but no '\n')

		if err == io.EOF && len(line) > 0 {
			return offset, nil
		}


		// Case: clean EOF (no more bytes)

		if err == io.EOF && len(line) == 0 {
			return offset, nil
		}

		// Case: real read error

		if err != nil {
			return offset, err
		}

		// Strip trailing '\n'

		line = line[:len(line)-1]

		// Give the line to the caller
		caller := fn(line);
		if caller != nil {
			return offset, nil
		}

		// Advance offset past this good record (including the '\n')
		offset += int64(len(line) + 1);
	}


}


// function for the truncating the file, used after a bad replay 
// to drop a partially-written record.

func (w *WAL) Truncate(size int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.file.Truncate(size); err != nil {
		return err
	}
	// Move the file position back to the new end so the next
	// Append writes immediately after the truncation point.
	_, err := w.file.Seek(0, io.SeekEnd)
	return err
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}