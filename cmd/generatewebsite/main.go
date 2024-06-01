// go run main.go ../website/pages html ../website/pages/sitemap.md

package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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

		// Write the content to the file
		fmt.Printf("Writing /%s\n", path)

		if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write to file %s: %w", filePath, err)
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

func processFile(path, srcDir, destDir string, info os.FileInfo, pageMap map[string]*greener.Page, rootSection *greener.Section, emptyPageProvider greener.EmptyPageProvider) error {
	relPath, err := filepath.Rel(srcDir, path)
	if err != nil {
		return err
	}
	targetRelPath := strings.TrimSuffix(relPath, ".md") + ".html"
	targetPath := filepath.Join(destDir, targetRelPath)

	// Ensure target directory exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	if strings.HasSuffix(info.Name(), ".md") {
		targetInfo, err := os.Stat(targetPath)
		if err == nil && targetInfo.ModTime().After(info.ModTime()) {
			// Target file is newer, skip processing
			fmt.Printf("Skipping %s\n", relPath)
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
		if err == nil && targetInfo.ModTime().After(info.ModTime()) {
			// Target file is newer, skip linking
			fmt.Printf("Skipping %s\n", relPath)
			return nil
		}

		// Create a hard link for other files
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
	manifestInjector, err := greener.NewDefaultManifestInjector(logger, appShortName, themeColor, "/start", shortCacheSeconds, iconInjector.IconPaths(), []int{192, 512})
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
	writeResponses(destDir, mux)
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return processFile(path, srcDir, destDir, info, pageMap, rootSection, emptyPageProvider)
		}
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing files: %v\n", err)
		os.Exit(1)
	}
}
