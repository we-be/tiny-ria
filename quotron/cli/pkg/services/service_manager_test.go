package services

import (
	"testing"
)

func TestServiceManagerCreateDestroy(t *testing.T) {
	// Create a config for testing
	config := DefaultConfig()
	
	// Create a service manager
	manager := NewServiceManager(config)
	if manager == nil {
		t.Fatal("Failed to create service manager")
	}
}

func TestCheckServiceRunning(t *testing.T) {
	// Create a config for testing
	config := DefaultConfig()
	
	// Create a service manager
	manager := NewServiceManager(config)
	
	// Test checking for non-existent service
	// This won't actually start anything
	isRunning := manager.checkServiceRunning("/tmp/non-existent-pid-file", "non-existent-pattern", "", 0)
	if isRunning {
		t.Error("checkServiceRunning should return false for non-existent service")
	}
}

func TestPIDList(t *testing.T) {
	// Test the PID list management
	
	// Clear the PID list before testing
	pidList = []int{}
	
	// Add some PIDs
	addPid(1000)
	addPid(2000)
	
	// Check that the PIDs were added
	if len(pidList) != 2 {
		t.Errorf("PID list length mismatch: got %d, want %d", len(pidList), 2)
	}
	
	if pidList[0] != 1000 || pidList[1] != 2000 {
		t.Errorf("PID list content mismatch: got %v, want [1000 2000]", pidList)
	}
}