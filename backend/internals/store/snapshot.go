package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Save writes the entire state to a snapshot file, atomically.

func Save(path string, data map[string]string) error {

	tmpPath := path + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(f)

	for k, v := range data {
		if _, err := fmt.Fprintf(w, `{"key":%q,"value":%q}`+"\n", k, v); err != nil {
			f.Close()
			return err
		}
	}

	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}

	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	_ = fsyncDir(filepath.Dir(path))
	return nil

}

func Load(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	result := make(map[string]string)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()

		var rec struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, err
		}
		result[rec.Key] = rec.Value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
