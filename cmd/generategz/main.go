package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func shouldIgnorePath(path string, ignoredDirs []string) bool {
	for _, dir := range ignoredDirs {
		if strings.HasPrefix(path, dir) {
			return true
		}
	}
	return false
}

func compressFile(path string) error {

	originalFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer originalFile.Close()

	origInfo, err := originalFile.Stat()
	if err != nil {
		return err
	}

	gzipPath := path + ".gz"
	gzipFile, err := os.Create(gzipPath)
	if err != nil {
		return err
	}
	defer gzipFile.Close()

	gw := gzip.NewWriter(gzipFile)
	if _, err := io.Copy(gw, originalFile); err != nil {
		return err
	}

	if err := gw.Close(); err != nil {
		return err
	}

	// Check sizes after gzipping
	gzipInfo, err := gzipFile.Stat()
	if err != nil || gzipInfo.Size() >= origInfo.Size() {
		fmt.Printf("Gzipped version is larger, so we'll delete it again\n")
		os.Remove(gzipPath) // Remove the gzip file if it's not smaller
		return nil
	}

	return nil
}

func main() {
	ignoredDirs := make([]string, 0)
	www := os.Args[1]
	for _, arg := range os.Args[2:] { // Start from 1 to skip the program name and directory
		// Convert relative directory argument to a path under "www"
		ignorePath := filepath.Join(www, arg)
		ignoredDirs = append(ignoredDirs, ignorePath)
	}

	fmt.Printf("Walking '%s' ...\n", www)
	filepath.Walk(www, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if shouldIgnorePath(path, ignoredDirs) || filepath.Ext(path) == "gz" {
			fmt.Printf("Ignoring '%s'\n", path)
			return err // Also return early if path should be ignored
		}

		gzipPath := path + ".gz"
		gzipInfo, err := os.Stat(gzipPath)
		if err == nil && !info.ModTime().After(gzipInfo.ModTime()) {
			// The compressed file is up-to-date or newer
			return nil
		}

		fmt.Printf("Compressing '%s' ...\n", path)
		return compressFile(path)
	})
}
