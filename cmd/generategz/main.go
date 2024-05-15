package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// Set of non-compressible MIME types
var nonCompressibleMIMETypes = map[string]struct{}{
	"application/gzip":             {},
	"application/zip":              {},
	"application/x-rar-compressed": {},
	"application/x-7z-compressed":  {},
	"application/x-tar":            {},
	"application/x-bzip2":          {},
	"application/x-xz":             {},
	"application/x-iso9660-image":  {},
	"image/jpeg":                   {},
	"image/png":                    {},
	"image/gif":                    {},
	"image/bmp":                    {},
	"image/webp":                   {},
	"image/svg+xml":                {},
	"video/mp4":                    {},
	"video/x-matroska":             {},
	"video/x-msvideo":              {},
	"video/quicktime":              {},
	"video/x-ms-wmv":               {},
	"audio/mpeg":                   {},
	"audio/aac":                    {},
	"audio/ogg":                    {},
	"audio/flac":                   {},
	"audio/wav":                    {},
	"audio/x-ms-wma":               {},
	"application/octet-stream":     {}, // Covers .exe, .dll, etc.
	"application/pdf":              {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   {},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         {},
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": {},
	"application/java-archive": {},
}

func shouldIgnorePath(path string, ignoredDirs []string) bool {
	for _, dir := range ignoredDirs {
		if strings.HasPrefix(path, dir) {
			return true
		}
	}
	return false
}

func shouldCompress(path string) bool {
	ext := filepath.Ext(path)
	mimeType := mime.TypeByExtension(ext)
	if _, found := nonCompressibleMIMETypes[mimeType]; found {
		return false
	}
	return true
}

func compressFile(srcPath, destPath string) error {
	originalFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", srcPath, err)
	}
	defer originalFile.Close()

	origInfo, err := originalFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %v", srcPath, err)
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", destPath, err)
	}
	defer destFile.Close()

	gw, err := gzip.NewWriterLevel(destFile, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("failed to create gzip writer for %s: %v", destPath, err)
	}

	if _, err := io.Copy(gw, originalFile); err != nil {
		return fmt.Errorf("failed to compress file %s: %v", srcPath, err)
	}

	if err := gw.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer for %s: %v", destPath, err)
	}

	// Check sizes after gzipping
	destInfo, err := destFile.Stat()
	if err != nil || destInfo.Size() >= origInfo.Size() {
		fmt.Printf("Gzipped version of %s is larger or an error occurred, truncating\n", srcPath)
		if err := destFile.Truncate(0); err != nil {
			return fmt.Errorf("failed to truncate file %s: %v", destPath, err)
		}
	}

	return nil
}

func ensureDirPermissions(srcDir, destDir string) error {
	srcInfo, err := os.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("failed to stat source directory %s: %v", srcDir, err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", srcDir)
	}

	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		// Destination directory does not exist, create it with the same permissions
		if err := os.MkdirAll(destDir, srcInfo.Mode()); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", destDir, err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to stat destination directory %s: %v", destDir, err)
	} else {
		// Destination directory exists, ensure it has the same permissions
		if srcInfo.Mode() != srcInfo.Mode() {
			if err := os.Chmod(destDir, srcInfo.Mode()); err != nil {
				return fmt.Errorf("failed to chmod directory %s: %v", destDir, err)
			}
		}
	}

	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: program <www directory> [ignored directories...]")
		return
	}

	ignoredDirs := make([]string, 0)
	www := os.Args[1]
	wwwgz := www + "gz"

	for _, arg := range os.Args[2:] {
		ignorePath := filepath.Join(www, arg)
		ignoredDirs = append(ignoredDirs, ignorePath)
	}

	fmt.Printf("Walking '%s' ...\n", www)
	err := filepath.Walk(www, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			relPath, err := filepath.Rel(www, path)
			if err != nil {
				return err
			}
			destDir := filepath.Join(wwwgz, relPath)
			return ensureDirPermissions(path, destDir)
		}

		if shouldIgnorePath(path, ignoredDirs) {
			fmt.Printf("Ignoring '%s' (it is in an ignored directory)\n", path)
			return nil
		}
		if filepath.Ext(path) == ".gz" {
			fmt.Printf("Ignoring '%s' since it has an extensions suggesting it is already gzipped\n", path)
			return nil
		}

		if !shouldCompress(path) {
			fmt.Printf("Ignoring '%s' due to its MIME type suggesting it won't compress well\n", path)
			return nil
		}

		relPath, err := filepath.Rel(www, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(wwwgz, relPath)

		destDir := filepath.Dir(destPath)
		if err := ensureDirPermissions(filepath.Dir(path), destDir); err != nil {
			return err
		}

		destInfo, err := os.Stat(destPath)
		if err == nil && !info.ModTime().After(destInfo.ModTime()) {
			fmt.Printf("Ignoring '%s' since it is already up to date\n", path)
			return nil
		}

		fmt.Printf("Compressing '%s' to '%s' ...\n", path, destPath)
		return compressFile(path, destPath)
	})

	if err != nil {
		fmt.Printf("Error walking the path %s: %v\n", www, err)
	}
}
