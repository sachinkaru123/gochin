package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// This script creates a symbolic link from /usr/local/bin/gochin 
// to the development version. This way you don't need to reinstall
// after each change.

func main() {
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	
	// Check if the path contains CHINGO or gochin to validate we're in the right directory
	if !strings.Contains(strings.ToLower(cwd), "gochin") && !strings.Contains(strings.ToLower(cwd), "chingo") {
		fmt.Println("Error: This script must be run from the gochin project directory")
		os.Exit(1)
	}
	
	// Define paths
	execPath := filepath.Join(cwd, "gochin")
	symlinkPath := "/usr/local/bin/gochin"
	
	// First make sure we build the binary
	fmt.Println("Building gochin binary...")
	buildCmd := exec.Command("go", "build", "-o", "gochin", "cmd/gochin/main.go")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	err = buildCmd.Run()
	if err != nil {
		fmt.Println("Error building gochin binary:", err)
		os.Exit(1)
	}
	
	// Check if the binary exists
	if _, err := os.Stat(execPath); os.IsNotExist(err) {
		fmt.Printf("Error: Could not find %s\n", execPath)
		os.Exit(1)
	}
	
	// Remove existing symlink if it exists
	if _, err := os.Lstat(symlinkPath); err == nil {
		fmt.Println("Removing existing symlink...")
		_ = os.Remove(symlinkPath)
	}
	
	// Create symlink
	fmt.Printf("Creating symlink from %s to %s\n", execPath, symlinkPath)
	linkCmd := exec.Command("sudo", "ln", "-s", execPath, symlinkPath)
	linkCmd.Stdout = os.Stdout
	linkCmd.Stderr = os.Stderr
	err = linkCmd.Run()
	if err != nil {
		fmt.Println("Error creating symlink - did you run with sudo?", err)
		os.Exit(1)
	}
	
	fmt.Println("\n✅ Success! Now when you run 'gochin', it will use your development version.")
	fmt.Println("When you make changes to code, just run 'make build' and your changes will be reflected.")
	fmt.Println("\nIMPORTANT: HTML and template files will be reloaded automatically without rebuilding.")
}

func runCommand(name string, args ...string) *os.Process {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	return cmd.Process
}