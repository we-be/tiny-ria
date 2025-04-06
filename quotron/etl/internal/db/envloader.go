package db

import (
	"os"
	"path/filepath"
	"strings"
)

// Try to load environment variables from .env files
func init() {
	// Try loading from .env file
	loadEnvFile(".env")
	
	// Try loading from ../.env (one directory up)
	loadEnvFile("../.env")
	
	// Try loading from ../../.env (two directories up)
	loadEnvFile("../../.env")
}

// loadEnvFile loads environment variables from a .env file
func loadEnvFile(path string) {
	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		// Try absolute path if relative doesn't exist
		if !filepath.IsAbs(path) {
			// Get current working directory
			wd, err := os.Getwd()
			if err == nil {
				absPath := filepath.Join(wd, path)
				if _, err := os.Stat(absPath); err == nil {
					path = absPath
				} else {
					return
				}
			} else {
				return
			}
		} else {
			return
		}
	}
	
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	
	// Parse lines
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Split by first equals sign
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Remove quotes if present
		value = strings.Trim(value, "\"'")
		
		// Only set if not already set
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}