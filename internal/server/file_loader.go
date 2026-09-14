package server

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// FileCacheEntry represents a cached file
type FileCacheEntry struct {
	Path    string
	Content []byte
	ModTime time.Time
}

// FileCache is an in-memory cache of files with timestamps
var FileCache = make(map[string]FileCacheEntry)

// GetHTMLContent returns the content of an HTML file, checking for changes
func GetHTMLContent(filename string) ([]byte, error) {
	// First find the path
	htmlPath, err := getHTMLPath(filename)
	if err != nil {
		return nil, err
	}

	// Get file info for modification time
	fileInfo, err := os.Stat(htmlPath)
	if err != nil {
		return nil, err
	}

	// Check cache - if file hasn't changed, return cached content
	if entry, exists := FileCache[filename]; exists {
		if entry.Path == htmlPath && entry.ModTime.Equal(fileInfo.ModTime()) {
			return entry.Content, nil
		}
	}

	// File is new or changed, read it
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		return nil, err
	}

	// Update cache
	FileCache[filename] = FileCacheEntry{
		Path:    htmlPath,
		Content: content,
		ModTime: fileInfo.ModTime(),
	}

	log.Printf("Loaded %s (modified: %s)", htmlPath, fileInfo.ModTime().Format(time.RFC3339))
	return content, nil
}

// getHTMLPath returns the path to the HTML file
// This tries multiple locations to handle both development and production environments
func getHTMLPath(filename string) (string, error) {
	// PRIORITIZE: Always check original source location first, to support live editing

	// Try approach #1: Check relative to current directory (repo root during development)
	cwd, err := os.Getwd()
	if err == nil {
		paths := []string{
			filepath.Join(cwd, "internal", "server", filename), // From repo root
			filepath.Join(cwd, "server", filename),             // From a different dir
		}

		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	// Try approach #2: Use source code directory (for development)
	_, sourcefile, _, ok := runtime.Caller(0)
	if ok {
		// Get the directory of the current source file
		dir := filepath.Dir(sourcefile)
		htmlPath := filepath.Join(dir, filename)

		if _, err := os.Stat(htmlPath); err == nil {
			return htmlPath, nil
		}
	}

	// Try approach #3: Check relative to executable
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		paths := []string{
			filepath.Join(exeDir, "internal", "server", filename),       // From executable dir
			filepath.Join(exeDir, "..", "internal", "server", filename), // Up one level
			filepath.Join(exeDir, "public", filename),                   // Public dir
			filepath.Join(exeDir, filename),                             // Direct in executable dir
		}

		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	// Nothing found, will need to return error
	return "", os.ErrNotExist
}
