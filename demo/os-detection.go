package main

import (
	"fmt"
	"path/filepath"
	"runtime"
)

// This demo shows how runtime.GOOS works across different platforms
func main() {
	fmt.Println("🔍 Gochin Framework - OS Detection Demo")
	fmt.Println("=====================================")
	
	// Current system detection
	fmt.Printf("Current System:\n")
	fmt.Printf("  OS: %s\n", runtime.GOOS)
	fmt.Printf("  Architecture: %s\n", runtime.GOARCH)
	fmt.Printf("  CPU Count: %d\n", runtime.NumCPU())
	fmt.Println()

	// Show what would happen on different systems
	fmt.Println("Installation Paths by OS:")
	
	systems := []struct {
		os   string
		arch string
		desc string
	}{
		{"linux", "amd64", "Ubuntu/Debian/CentOS/etc (64-bit)"},
		{"linux", "386", "Linux 32-bit"},
		{"linux", "arm64", "Linux ARM64 (Raspberry Pi, etc.)"},
		{"darwin", "amd64", "macOS Intel (64-bit)"},
		{"darwin", "arm64", "macOS Apple Silicon (M1/M2)"},
		{"windows", "amd64", "Windows 64-bit"},
		{"windows", "386", "Windows 32-bit"},
		{"freebsd", "amd64", "FreeBSD 64-bit"},
	}

	for _, sys := range systems {
		installPath := getInstallPathForOS(sys.os)
		binaryName := getBinaryNameForOS(sys.os)
		
		fmt.Printf("  %s/%s (%s):\n", sys.os, sys.arch, sys.desc)
		fmt.Printf("    Binary: %s\n", binaryName)
		fmt.Printf("    Install to: %s\n", installPath)
		fmt.Printf("    Build command: CGO_ENABLED=0 GOOS=%s GOARCH=%s go build -o %s ./cmd/gochin\n", 
			sys.os, sys.arch, binaryName)
		fmt.Println()
	}

	// Current system specific info
	fmt.Printf("For your current system (%s/%s):\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Binary name: %s\n", getBinaryNameForOS(runtime.GOOS))
	fmt.Printf("  Install path: %s\n", getInstallPathForOS(runtime.GOOS))
	fmt.Printf("  Requires sudo: %v\n", requiresSudo(runtime.GOOS))
	
	fmt.Println()
	fmt.Println("💡 How it works:")
	fmt.Println("  • runtime.GOOS returns the target operating system")
	fmt.Println("  • runtime.GOARCH returns the target architecture")
	fmt.Println("  • We use these to determine the correct installation path")
	fmt.Println("  • No need for manual OS detection or complex scripts!")
}

// getBinaryNameForOS returns the binary name for a given OS
func getBinaryNameForOS(goos string) string {
	if goos == "windows" {
		return "gochin.exe"
	}
	return "gochin"
}

// getInstallPathForOS returns the installation path for a given OS
func getInstallPathForOS(goos string) string {
	binaryName := getBinaryNameForOS(goos)
	
	switch goos {
	case "linux", "darwin", "freebsd", "netbsd", "openbsd":
		return filepath.Join("/usr/local/bin", "gochin")
	case "windows":
		return filepath.Join("C:\\Program Files\\Gochin", binaryName)
	default:
		return "./gochin"
	}
}

// requiresSudo indicates if the OS typically requires sudo for installation
func requiresSudo(goos string) bool {
	switch goos {
	case "linux", "darwin", "freebsd", "netbsd", "openbsd":
		return true
	case "windows":
		return false // Uses UAC instead
	default:
		return false
	}
}