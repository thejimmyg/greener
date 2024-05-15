package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// ETagEntry represents an entry in the etag.json file
type ETagEntry struct {
	MTime time.Time `json:"mtime"`
	ETag  string    `json:"etag"`
}

// ETagFile represents the etag.json file
type ETagFile struct {
	Entries map[string]ETagEntry `json:"entries"`
}

// LoadETagFile loads the etag.json file if it exists
func LoadETagFile(etagFilePath string) (*ETagFile, error) {
	file, err := os.Open(etagFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ETagFile{Entries: make(map[string]ETagEntry)}, nil
		}
		return nil, err
	}
	defer file.Close()

	var etagFile ETagFile
	if err := json.NewDecoder(file).Decode(&etagFile); err != nil {
		return nil, err
	}

	return &etagFile, nil
}

// SaveETagFile saves the etag.json file
func SaveETagFile(etagFilePath string, etagFile *ETagFile) error {
	file, err := os.Create(etagFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(etagFile)
}

// GenerateETag generates an ETag for a file
func GenerateETag(filePath string, h hash.Hash) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// UpdateETags updates the ETags for the files in the www directory
func UpdateETags(wwwDir string, etagFile *ETagFile, salt []byte) error {
	h := hmac.New(sha256.New, salt)

	err := filepath.Walk(wwwDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(wwwDir, path)
		if err != nil {
			return err
		}

		entry, exists := etagFile.Entries[relPath]
		if !exists || info.ModTime().After(entry.MTime) {
			h.Reset()
			etag, err := GenerateETag(path, h)
			if err != nil {
				return err
			}
			etagFile.Entries[relPath] = ETagEntry{
				MTime: info.ModTime(),
				ETag:  etag,
			}
			fmt.Printf("Updated ETag for %s: %s\n", relPath, etag)
		}

		return nil
	})

	return err
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: generateetags <www directory path> <etag.json path> [salt]")
		return
	}

	wwwDir := os.Args[1]
	etagFilePath := os.Args[2]
	var salt []byte
	if len(os.Args) >= 4 {
		salt = []byte(os.Args[3])
	}

	etagFile, err := LoadETagFile(etagFilePath)
	if err != nil {
		fmt.Printf("Error loading etag file: %v\n", err)
		return
	}

	err = UpdateETags(wwwDir, etagFile, salt)
	if err != nil {
		fmt.Printf("Error updating etags: %v\n", err)
		return
	}

	err = SaveETagFile(etagFilePath, etagFile)
	if err != nil {
		fmt.Printf("Error saving etag file: %v\n", err)
		return
	}
}
