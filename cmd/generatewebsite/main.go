// touch static/icon-1
// mkdir -p static/other/one
// go run main.go ../website/pages static ../website/pages/sitemap.md ../website/ icon-512x512.png

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/thejimmyg/greener"
)

func writeResponses(targetDir string, mux *greener.CaptureMux) error {
	for path, content := range mux.Responses {
		// Remove the first character from the path
		if len(path) > 1 {
			path = path[1:]
		} else {
			return fmt.Errorf("path '/' too short to write to file")
		}

		// Create the full file path
		filePath := filepath.Join(targetDir, path)

		// Ensure the directory exists
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create directories for path %s: %w", filePath, err)
		}

		// Check if the file exists and compare the content
		currentContent, err := ioutil.ReadFile(filePath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to read file %s: %w", filePath, err)
		}

		// Write the content to the file only if it's different or doesn't exist
		if os.IsNotExist(err) || !bytes.Equal(currentContent, []byte(content)) {
			fmt.Printf("Writing /%s\n", path)
			if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write to file %s: %w", filePath, err)
			}
		} else {
			fmt.Printf("Skipping unchanged file %s\n", path)
		}
	}
	return nil
}

func transform(input []byte, currentPath string, pageMap map[string]*greener.Page, rootSection *greener.Section, emptyPageProvider greener.EmptyPageProvider) (template.HTML, error) {
	if p, ok := pageMap[currentPath]; ok {
		mdHTML, err := greener.ConvertMarkdownToHTML(input, currentPath)
		if err != nil {
			return mdHTML, err
		}
		breadcrumbs := greener.GenerateBreadcrumbs(currentPath, p)
		sectionNav := greener.GenerateSectionNav(p, false, "section", currentPath)
		html := emptyPageProvider.Page(p.Title, greener.HTMLPrintf("%s%s%s", breadcrumbs, sectionNav, mdHTML), currentPath)
		return html, err
	}
	return template.HTML(""), fmt.Errorf("page not in sitemap: %v+\n%s\n", pageMap, currentPath)
}

func findExtraneousFiles(targetDir string, expectedFiles map[string]struct{}) ([]string, error) {
	var extraneousFiles []string
	err := filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if _, found := expectedFiles[path]; !found && !info.IsDir() {
			extraneousFiles = append(extraneousFiles, path)
		}
		return nil
	})
	return extraneousFiles, err
}

func findExtraneousDirs(destDir string, expectedDirs map[string]bool) ([]string, error) {
	var dirsToCheck []string
	var unneededDirs []string

	// First, collect all directories.
	err := filepath.WalkDir(destDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirsToCheck = append(dirsToCheck, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Check from the deepest directory upwards.
	sort.Sort(sort.Reverse(sort.StringSlice(dirsToCheck)))

	// Track empty status of directories
	emptyDirs := make(map[string]bool)
	for _, dir := range dirsToCheck {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		if !expectedDirs[dir] {
			isEmpty := true
			for _, entry := range entries {
				subPath := filepath.Join(dir, entry.Name())
				if entry.IsDir() {
					// Check if the subdirectory is marked as empty
					if !emptyDirs[subPath] {
						isEmpty = false
						break
					}
				} else {
					isEmpty = false
					break
				}
			}
			if isEmpty {
				unneededDirs = append(unneededDirs, dir)
				emptyDirs[dir] = true
			}
		}
	}

	return unneededDirs, nil
}

func processFile(path, srcDir, destDir string, info os.FileInfo, pageMap map[string]*greener.Page, rootSection *greener.Section, emptyPageProvider greener.EmptyPageProvider, expectedFiles map[string]struct{}, expectedDirs map[string]bool) error {
	dirPath := filepath.Dir(path)
	expectedDirs[dirPath] = true
	relPath, err := filepath.Rel(srcDir, path)
	if err != nil {
		return err
	}
	targetRelPath := strings.TrimSuffix(relPath, ".md") + ".html"
	targetPath := filepath.Join(destDir, targetRelPath)
	expectedFiles[targetPath] = struct{}{}

	// Ensure target directory exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	if strings.HasSuffix(info.Name(), ".md") {
		targetInfo, err := os.Stat(targetPath)
		if err == nil && targetInfo.ModTime().After(info.ModTime()) {
			// Target file is newer, skip processing
			fmt.Printf("Skipping not newer file %s\n", relPath)
			return nil
		}

		// Read the entire markdown file
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		// Apply transformation
		fmt.Printf("Transforming and writing %s\n", "/"+relPath)
		transformedData, err := transform(data, "/"+targetRelPath, pageMap, rootSection, emptyPageProvider)
		if err != nil {
			return err
		}

		// Write the result to the new HTML file
		return ioutil.WriteFile(targetPath, []byte(transformedData), 0644)
	} else {

		// Check if the target file exists and compare mod times
		targetInfo, err := os.Stat(targetPath)
		if err == nil {
			if targetInfo.ModTime().After(info.ModTime()) {
				// Target file is newer, skip linking
				fmt.Printf("Skipping %s\n", relPath)
				return nil
			}
			// Target file is older, remove it before linking
			fmt.Printf("Removing older %s\n", relPath)
			if err := os.Remove(targetPath); err != nil {
				fmt.Printf("Failed to remove %s: %v\n", targetPath, err)
				return err
			}
		} else if !os.IsNotExist(err) {
			// Some other error occurred when attempting to stat the file
			fmt.Printf("Error stating %s: %v\n", targetPath, err)
			return err
		}

		// Create a hard link for the file
		fmt.Printf("Linking %s\n", relPath)
		return os.Link(path, targetPath)

	}
}

func main() {
	if len(os.Args) != 6 {
		fmt.Println("Usage: <program> <source directory> <destination directory> <path to sitemap markdown file> <path to icon file dir> <relative icon file path>")
		return
	}

	srcDir := os.Args[1]
	destDir := os.Args[2]
	sitemapPath := os.Args[3]
	iconFileDir := os.Args[4]
	iconFileRelPath := os.Args[5]

	expectedFiles := make(map[string]struct{})
	expectedDirs := make(map[string]bool)

	pageMap := make(map[string]*greener.Page)
	markdown, err := ioutil.ReadFile(sitemapPath)
	if err != nil {
		log.Fatalf("Error reading sitemap markdown: %s", err)
	}
	rootSection, err := greener.ParseSitemap("/sitemap.html", markdown)
	if err != nil {
		log.Fatalf("Error parsing sitemap markdown: %s", err)
	}
	greener.BuildPageInSectionMap(rootSection, pageMap)

	themeColor := "#000000"
	logger := greener.NewDefaultLogger(log.Printf)
	// Both these would be longer for production though
	longCacheSeconds := 60 // In real life you might set this to a day, a month or a year perhaps
	shortCacheSeconds := 5 // Keep this fairly short because you want changes to propgagte quickly

	iconFileFS := os.DirFS(iconFileDir)
	iconInjector, err := greener.NewDefaultIconsInjector(logger, iconFileFS, iconFileRelPath, []int{16, 32, 144, 180, 192, 512}, longCacheSeconds)
	appShortName := "Static"
	if err != nil {
		panic(err)
	}
	manifestInjector, err := greener.NewDefaultManifestInjector(logger, appShortName, themeColor, "start", shortCacheSeconds, iconInjector.IconPaths(), []int{192, 512})
	if err != nil {
		panic(err)
	}
	injectors := []greener.Injector{
		greener.NewDefaultStyleInjector(logger, greener.NavUISupport, longCacheSeconds),
		greener.NewDefaultScriptInjector(logger, greener.NavUISupport, longCacheSeconds),
		greener.NewDefaultThemeColorInjector(logger, themeColor),
		greener.NewDefaultSEOInjector(logger, "A web app"),
		iconInjector,
		manifestInjector,
	}
	emptyPageProvider := greener.NewDefaultEmptyPageProvider(injectors)
	mux := greener.NewCaptureMux()
	emptyPageProvider.PerformInjections(mux)

	for path, _ := range mux.Responses {
		fullPath := filepath.Join(destDir, path[1:]) // Remove the first character which will be a '/'
		expectedFiles[fullPath] = struct{}{}
	}

	writeResponses(destDir, mux)
	walkingFailed := false
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("ERROR: %s\n", err)
			walkingFailed = true
			// return err
		}
		if !info.IsDir() {
			if err := processFile(path, srcDir, destDir, info, pageMap, rootSection, emptyPageProvider, expectedFiles, expectedDirs); err != nil {
				fmt.Printf("ERROR: %s\n", err)
				walkingFailed = true
				// return err
			}
		}
		return nil
	})

	// After processing all files, find extraneous files
	extraneousFiles, err := findExtraneousFiles(destDir, expectedFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding extraneous files: %v\n", err)
		os.Exit(1)
	}

	// Output extraneous files
	if len(extraneousFiles) > 0 {
		jsonFileData, err := json.Marshal(extraneousFiles)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling extraneous files to JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Extraneous files: %s\n", jsonFileData)
	}

	extraneousDirs, err := findExtraneousDirs(destDir, expectedDirs)
	if err != nil {
		log.Fatalf("Error finding unused directories: %s", err)
	}

	if len(extraneousDirs) > 0 {
		jsonDirData, err := json.Marshal(extraneousDirs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling extraneous dirs to JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Extraneous directories: %s\n", jsonDirData)
	}

	if len(extraneousFiles) > 0 {
		fmt.Println("# Shell command to remove extraneous files:")
		for _, file := range extraneousFiles {
			fmt.Printf("rm %s\n", strconv.Quote(file))
		}
	}
	if len(extraneousDirs) > 0 {
		fmt.Println("# Shell command to remove extraneous dirs:")
		for _, dir := range extraneousDirs {
			fmt.Printf("rmdir %s\n", strconv.Quote(dir))
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing files: %v\n", err)
		os.Exit(1)
	} else if walkingFailed == true {
		fmt.Fprintf(os.Stderr, "Error processing files. See error messagages above.\n")
		os.Exit(1)
	}
}
