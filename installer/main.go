package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	fmt.Println("🚀 Gochin Framework - Smart Installer")
	fmt.Printf("Detected OS: %s\n", runtime.GOOS)
	fmt.Printf("Detected Architecture: %s\n", runtime.GOARCH)
	fmt.Println()

	// Build the binary first
	fmt.Println("📦 Building Gochin binary...")
	if err := buildBinary(); err != nil {
		fmt.Printf("❌ Build failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Build successful")

	// Install based on detected OS
	fmt.Println("📋 Installing Gochin globally...")
	if err := installBinary(); err != nil {
		fmt.Printf("❌ Installation failed: %v\n", err)
		os.Exit(1)
	}

	// Verify installation
	fmt.Println("🔍 Verifying installation...")
	if err := verifyInstallation(); err != nil {
		fmt.Printf("⚠️  Verification failed: %v\n", err)
		fmt.Println("The binary was copied but may not be in your PATH")
	} else {
		fmt.Println("✅ Installation verified successfully!")
	}

	printUsageInstructions()
}

// buildBinary compiles the Gochin CLI for the current platform
func buildBinary() error {
	binaryName := getBinaryName()
	cmdDir := "./cmd/gochin"

	cmd := exec.Command("go", "build", "-o", binaryName, cmdDir)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build command failed: %v\nOutput: %s", err, output)
	}

	return nil
}

// installBinary copies the binary to the appropriate system location
func installBinary() error {
	binaryName := getBinaryName()
	installPath := getInstallPath()

	switch runtime.GOOS {
	case "linux", "darwin": // Linux or macOS
		return installUnix(binaryName, installPath)
	case "windows":
		return installWindows(binaryName, installPath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// installUnix handles installation on Unix-like systems (Linux/macOS)
func installUnix(binaryName, installPath string) error {
	// Check if we have write permissions to the install directory
	if err := checkWritePermission(filepath.Dir(installPath)); err != nil {
		fmt.Println("⚠️  Need sudo permissions for installation")
		return installWithSudo(binaryName, installPath)
	}

	// Copy the binary
	return copyFile(binaryName, installPath)
}

// installWindows handles installation on Windows
func installWindows(binaryName, installPath string) error {
	fmt.Printf("Installing to: %s\n", installPath)
	
	// Try to copy directly first
	if err := copyFile(binaryName, installPath); err != nil {
		// If that fails, provide manual instructions
		fmt.Println("❌ Automatic installation failed")
		fmt.Printf("Please manually copy %s to a directory in your PATH\n", binaryName)
		fmt.Println("Common locations:")
		fmt.Println("  • C:\\Windows\\System32\\")
		fmt.Println("  • C:\\Program Files\\Gochin\\ (then add to PATH)")
		return err
	}

	return nil
}

// installWithSudo uses sudo to copy the binary (Unix systems)
func installWithSudo(binaryName, installPath string) error {
	cmd := exec.Command("sudo", "cp", binaryName, installPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo copy failed: %v", err)
	}

	// Set execute permissions
	cmd = exec.Command("sudo", "chmod", "+x", installPath)
	return cmd.Run()
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Read source file
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %v", err)
	}

	// Write to destination
	if err := os.WriteFile(dst, data, 0755); err != nil {
		return fmt.Errorf("failed to write destination file: %v", err)
	}

	return nil
}

// checkWritePermission checks if we can write to a directory
func checkWritePermission(dir string) error {
	testFile := filepath.Join(dir, ".gochin_write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return err
	}
	f.Close()
	os.Remove(testFile)
	return nil
}

// getBinaryName returns the appropriate binary name for the current OS
func getBinaryName() string {
	if runtime.GOOS == "windows" {
		return "gochin.exe"
	}
	return "gochin"
}

// getInstallPath returns the appropriate installation path for the current OS
func getInstallPath() string {
	binaryName := getBinaryName()
	
	switch runtime.GOOS {
	case "linux", "darwin":
		return filepath.Join("/usr/local/bin", "gochin")
	case "windows":
		// Try Program Files first, fall back to System32
		programFiles := os.Getenv("PROGRAMFILES")
		if programFiles != "" {
			gochinDir := filepath.Join(programFiles, "Gochin")
			return filepath.Join(gochinDir, binaryName)
		}
		return filepath.Join("C:\\Windows\\System32", binaryName)
	default:
		return "./gochin"
	}
}

// verifyInstallation checks if the gochin command is available in PATH
func verifyInstallation() error {
	cmd := exec.Command("gochin", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command not found in PATH: %v", err)
	}
	
	fmt.Printf("Installed version: %s", output)
	return nil
}

// printUsageInstructions shows how to use the installed CLI
func printUsageInstructions() {
	fmt.Println()
	fmt.Println("🎉 Gochin Framework installed successfully!")
	fmt.Println()
	fmt.Println("📚 Quick start commands:")
	fmt.Println("  gochin --help")
	fmt.Println("  gochin run start")
	fmt.Println("  gochin make controller User")
	fmt.Println("  gochin db migrate")
	fmt.Println()
	
	if runtime.GOOS == "windows" {
		fmt.Println("Note: On Windows, you may need to restart your command prompt")
		fmt.Println("or add the installation directory to your PATH environment variable.")
	} else {
		fmt.Println("Note: If 'gochin' command is not found, you may need to:")
		fmt.Println("  • Restart your terminal")
		fmt.Println("  • Run: source ~/.bashrc (or ~/.zshrc)")
	}
}