package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ServiceList defines which services should be operated on
type ServiceList struct {
	YFinanceProxy bool
	APIService    bool
	Scheduler     bool
	ETLService    bool
}

// ServiceStatus represents the running status of each service
type ServiceStatus struct {
	YFinanceProxy bool
	APIService    bool
	Scheduler     bool
	ETLService    bool
}

// ServiceManager manages operations on services
type ServiceManager struct {
	config *Config
}

// NewServiceManager creates a new ServiceManager
func NewServiceManager(config *Config) *ServiceManager {
	return &ServiceManager{
		config: config,
	}
}

// StartServices starts the specified services
func (sm *ServiceManager) StartServices(ctx context.Context, services ServiceList, monitor bool) error {
	// The defer cleanup was causing services to stop after starting
	// We should only clean up on abnormal exit, not in normal operation

	// Build start order based on dependencies
	if services.APIService && !services.YFinanceProxy {
		// API service requires YFinance proxy
		services.YFinanceProxy = true
	}

	// Start services in order
	if services.YFinanceProxy {
		err := sm.startYFinanceProxy(ctx)
		if err != nil {
			return fmt.Errorf("failed to start YFinance Proxy: %w", err)
		}
	}

	if services.APIService {
		err := sm.startAPIService(ctx)
		if err != nil {
			return fmt.Errorf("failed to start API Service: %w", err)
		}
	}

	if services.Scheduler {
		err := sm.startScheduler(ctx)
		if err != nil {
			return fmt.Errorf("failed to start Scheduler: %w", err)
		}
	}


	if services.ETLService {
		err := sm.startETLService(ctx)
		if err != nil {
			return fmt.Errorf("failed to start ETL Service: %w", err)
		}
	}

	// If monitor mode is enabled, start monitoring services
	if monitor {
		go sm.monitorServices(ctx, services)
	}

	return nil
}

// StopServices stops the specified services
func (sm *ServiceManager) StopServices(services ServiceList) error {
	// Stop services in reverse dependency order

	if services.Scheduler {
		err := sm.stopService("Scheduler", sm.config.SchedulerPIDFile, "scheduler")
		if err != nil {
			return fmt.Errorf("failed to stop Scheduler: %w", err)
		}
	}

	if services.APIService {
		err := sm.stopService("API Service", sm.config.APIServicePIDFile, "api-service")
		if err != nil {
			return fmt.Errorf("failed to stop API Service: %w", err)
		}
	}

	if services.ETLService {
		err := sm.stopService("ETL Service", sm.config.ETLServicePIDFile, "etl.*-start")
		if err != nil {
			return fmt.Errorf("failed to stop ETL Service: %w", err)
		}
	}

	if services.YFinanceProxy {
		// Use the kill script for reliable termination
		killScript := filepath.Join(sm.config.QuotronRoot, "api-scraper", "scripts", "kill_proxy.sh")
		if _, statErr := os.Stat(killScript); os.IsNotExist(statErr) {
			fmt.Printf("Kill script not found at %s, creating it...\n", killScript)
			// Create a minimal kill script on the fly
			killContent := `#!/bin/bash
echo "Stopping all YFinance proxy processes..."
pkill -9 -f "python.*yfinance_proxy.py" || true
rm -f /tmp/yfinance_proxy.pid
echo "Done"
`
			os.WriteFile(killScript, []byte(killContent), 0755)
		}
		
		// Make sure it's executable
		os.Chmod(killScript, 0755)
		
		// Run the kill script
		fmt.Println("Forcefully stopping all YFinance Proxy processes...")
		stopCmd := exec.Command(killScript)
		stopCmd.Stdout = os.Stdout
		stopCmd.Stderr = os.Stderr
		stopCmd.Run() // Ignore errors, we want to continue regardless
		
		// Verify no processes are left
		time.Sleep(1 * time.Second)
		checkCmd := exec.Command("pgrep", "-f", "python.*yfinance_proxy.py")
		if checkCmd.Run() == nil {
			fmt.Println("WARNING: Some YFinance Proxy processes may still be running")
		} else {
			fmt.Println("All YFinance Proxy processes stopped successfully")
		}
	}

	return nil
}

// GetServiceStatus returns the status of all services
func (sm *ServiceManager) GetServiceStatus() (*ServiceStatus, error) {
	status := &ServiceStatus{
		YFinanceProxy: false,
		APIService:    false,
		Scheduler:     false,
		ETLService:    false,
	}

	// Check YFinance Proxy
	status.YFinanceProxy = sm.checkServiceRunning(sm.config.YFinanceProxyPIDFile, "python.*yfinance_proxy.py",
		sm.config.YFinanceProxyHost, sm.config.YFinanceProxyPort)

	// Check API Service
	status.APIService = sm.checkServiceRunning(sm.config.APIServicePIDFile, "api-service",
		sm.config.APIHost, sm.config.APIPort)

	// Check Scheduler - Check PID file first, then look for process
	status.Scheduler = false
	
	// First try direct PID check
	pid, err := sm.readPid(sm.config.SchedulerPIDFile)
	if err == nil && pid > 0 && isPidRunning(pid) {
		status.Scheduler = true
	} else {
		// If PID check fails, check for any scheduler process
		cmd := exec.Command("pgrep", "-f", "scheduler")
		if cmd.Run() == nil {
			status.Scheduler = true
		}
	}

		
	// Check ETL Service
	// First check PID file
	etlPid, err := sm.readPid(sm.config.ETLServicePIDFile)
	if err == nil && etlPid > 0 && isPidRunning(etlPid) {
		status.ETLService = true
	} else {
		// Then check for any ETL processes running
		cmd := exec.Command("pgrep", "-f", "etl.*-start")
		if cmd.Run() == nil {
			status.ETLService = true
		}
	}

	return status, nil
}

// startYFinanceProxy starts the YFinance proxy
func (sm *ServiceManager) startYFinanceProxy(ctx context.Context) error {
	// Use a simplified approach that's known to work from the command line
	
	// Check if already running
	if sm.checkServiceResponding(sm.config.YFinanceProxyHost, sm.config.YFinanceProxyPort) {
		fmt.Println("YFinance Proxy is already running and responding")
		return nil
	}
	
	// Stop any existing daemon process
	daemonPath := filepath.Join(sm.config.QuotronRoot, "api-scraper", "scripts", "daemon_proxy.sh")
	if _, statErr := os.Stat(daemonPath); os.IsNotExist(statErr) {
		fmt.Printf("Warning: daemon script not found at %s\n", daemonPath)
		// Fall back to traditional method if daemon script isn't available
		sm.stopService("YFinance Proxy", sm.config.YFinanceProxyPIDFile, "python.*yfinance_proxy.py")
	} else {
		fmt.Println("Stopping existing YFinance Proxy service...")
		stopCmd := exec.Command(daemonPath, "stop")
		stopCmd.Stdout = os.Stdout
		stopCmd.Stderr = os.Stderr
		_ = stopCmd.Run() // Ignore errors, we're stopping anyway
	}
	
	// Path setup
	scriptsDir := filepath.Join(sm.config.QuotronRoot, "api-scraper", "scripts")
	scriptPath := filepath.Join(scriptsDir, "yfinance_proxy.py")
	
	// Verify the script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("YFinance proxy script not found at %s", scriptPath)
	}
	
	// Use virtualenv if available
	pythonPath := "python3"
	venvPath := filepath.Join(sm.config.QuotronRoot, ".venv")
	if _, err := os.Stat(filepath.Join(venvPath, "bin", "python")); err == nil {
		pythonPath = filepath.Join(venvPath, "bin", "python")
		fmt.Printf("Using Python from virtualenv: %s\n", pythonPath)
	}
	
	// Set up log file
	logFile, err := os.OpenFile(sm.config.YFinanceLogFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()
	
	// Use the daemon script instead of directly running Python
	daemonPath = filepath.Join(scriptsDir, "daemon_proxy.sh")
	if _, statErr := os.Stat(daemonPath); os.IsNotExist(statErr) {
		return fmt.Errorf("daemon script not found at %s", daemonPath)
	}
	
	// Make script executable
	_ = os.Chmod(daemonPath, 0755)
	
	fmt.Println("Starting YFinance Proxy daemon...")
	cmd := exec.CommandContext(ctx, daemonPath, 
		"--host", sm.config.YFinanceProxyHost,
		"--port", strconv.Itoa(sm.config.YFinanceProxyPort))
	cmd.Dir = scriptsDir
	
	// Capture output directly to terminal
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Set health service URL in environment
	cmd.Env = append(os.Environ(), 
		fmt.Sprintf("HEALTH_SERVICE_URL=%s", sm.config.HealthServiceURL))
	
	// Run the daemon script (will run and wait for HTTP response)
	runErr := cmd.Run()
	if runErr != nil {
		// Check if the error is because the daemon is already running
		if sm.checkServiceResponding(sm.config.YFinanceProxyHost, sm.config.YFinanceProxyPort) {
			fmt.Println("YFinance proxy is already running and responding - continuing")
			fmt.Printf("UI available at http://%s:%d\n", sm.config.YFinanceProxyHost, sm.config.YFinanceProxyPort)
			return nil
		}
		
		// If we're here, it's a real error
		fmt.Printf("Error: Daemon script returned non-zero exit code: %v\n", runErr)
		logTail, _ := exec.Command("tail", "-n", "20", sm.config.YFinanceLogFile).Output()
		if len(logTail) > 0 {
			fmt.Printf("\nLast log entries:\n%s\n", string(logTail))
		}
		return fmt.Errorf("failed to start YFinance Proxy daemon: %w", runErr)
	}
	
	// If we're here, the daemon script has successfully started the proxy
	fmt.Printf("YFinance Proxy daemon started successfully\n")
	fmt.Printf("UI available at http://%s:%d\n", sm.config.YFinanceProxyHost, sm.config.YFinanceProxyPort)
	fmt.Printf("Log file: %s\n", sm.config.YFinanceLogFile)
	return nil
}


// startAPIService starts the API service with improved persistence
func (sm *ServiceManager) startAPIService(ctx context.Context) error {
	// Check if already running
	if sm.checkServiceResponding(sm.config.APIHost, sm.config.APIPort) {
		fmt.Println("API Service is already running and responding")
		return nil
	}

	// Ensure YFinance proxy is running
	if !sm.checkServiceRunning(sm.config.YFinanceProxyPIDFile, "python.*yfinance_proxy.py",
		sm.config.YFinanceProxyHost, sm.config.YFinanceProxyPort) {
		fmt.Println("YFinance Proxy is not running. Starting it now...")
		err := sm.startYFinanceProxy(ctx)
		if err != nil {
			return fmt.Errorf("failed to start YFinance Proxy: %w", err)
		}
	}

	// Build API service if needed
	apiServiceDir := filepath.Join(sm.config.QuotronRoot, "api-service")
	apiServiceBin := filepath.Join(apiServiceDir, "api-service")

	// Check if binary exists and is executable
	_, err := os.Stat(apiServiceBin)
	if os.IsNotExist(err) || !isExecutable(apiServiceBin) {
		fmt.Println("Building API Service...")
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "api-service", "cmd/server/main.go")
		buildCmd.Dir = apiServiceDir
		buildOut, err := buildCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to build API Service: %w, output: %s", err, buildOut)
		}
		fmt.Println("API Service built successfully")
	}

	// Start the API service
	fmt.Println("Starting API service...")
	
	// Ensure config dir exists
	configDir := filepath.Dir(sm.config.APIServicePIDFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Arguments
	args := []string{
		"--port", strconv.Itoa(sm.config.APIPort),
		"--yahoo-host", sm.config.YFinanceProxyHost,
		"--yahoo-port", strconv.Itoa(sm.config.YFinanceProxyPort),
	}
	
	// Add health service if enabled
	if sm.config.HealthServiceURL != "" {
		args = append(args, 
			"--health", "true",
			"--health-service", sm.config.HealthServiceURL)
	}
	
	// Prepare command
	cmd := exec.CommandContext(ctx, apiServiceBin, args...)
	cmd.Dir = apiServiceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Start the API service
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start API Service: %w", err)
	}
	
	// Save PID
	err = sm.savePid(sm.config.APIServicePIDFile, cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failed to save API Service PID: %w", err)
	}
	
	// Add to global PID list
	addPid(cmd.Process.Pid)
	
	// Wait for service to be responsive
	fmt.Printf("Waiting for service at %s:%d to respond (timeout: %ds)...\n", 
		sm.config.APIHost, sm.config.APIPort, 30)
	
	// Poll until service is responsive or timeout reached
	timeout := 30 * time.Second
	start := time.Now()
	attempts := 0
	
	for time.Since(start) < timeout {
		if sm.checkServiceResponding(sm.config.APIHost, sm.config.APIPort) {
			fmt.Printf("Service available after %.1f seconds (%d attempts)\n", 
				time.Since(start).Seconds(), attempts)
			break
		}
		
		attempts++
		time.Sleep(2 * time.Second)
		
		if attempts%5 == 0 {
			fmt.Printf("Still waiting for service to respond (%.1f seconds elapsed, %d attempts)...\n", 
				time.Since(start).Seconds(), attempts)
		}
	}
	
	if !sm.checkServiceResponding(sm.config.APIHost, sm.config.APIPort) {
		return fmt.Errorf("timed out waiting for API Service to respond")
	}
	
	fmt.Printf("API service started successfully with PID %d\n", cmd.Process.Pid)
	return nil
}

// startScheduler starts the scheduler
func (sm *ServiceManager) startScheduler(ctx context.Context) error {
	// Check if already running
	pid, err := sm.readPid(sm.config.SchedulerPIDFile)
	if err == nil && pid > 0 && isPidRunning(pid) {
		fmt.Println("Scheduler is already running")
		return nil
	}

	// Build scheduler if needed
	schedulerDir := filepath.Join(sm.config.QuotronRoot, "scheduler")
	schedulerBin := filepath.Join(schedulerDir, "scheduler")

	// Check if binary exists and is executable
	_, err = os.Stat(schedulerBin)
	if os.IsNotExist(err) || !isExecutable(schedulerBin) {
		fmt.Println("Building Scheduler...")
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "scheduler", "cmd/scheduler/main.go")
		buildCmd.Dir = schedulerDir
		buildOut, err := buildCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to build Scheduler: %w, output: %s", err, buildOut)
		}
	}

	// Start the scheduler
	fmt.Println("Starting Scheduler...")
	
	// Ensure data directory exists
	dataDir := filepath.Join(schedulerDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	
	// Configure scheduler
	configFile := sm.config.SchedulerConfigFile
	if configFile == "" {
		configFile = filepath.Join(sm.config.QuotronRoot, "scheduler-config.json")
	}
	
	// Check config file
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("scheduler config file not found at %s", configFile)
	}
	
	// Start the process
	cmd := exec.CommandContext(ctx, schedulerBin, "--config", configFile)
	cmd.Dir = schedulerDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Start the scheduler
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start Scheduler: %w", err)
	}
	
	// Save PID
	err = sm.savePid(sm.config.SchedulerPIDFile, cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failed to save Scheduler PID: %w", err)
	}
	
	// Add to global PID list
	addPid(cmd.Process.Pid)
	
	// Check if process is still running after a brief delay
	time.Sleep(1 * time.Second)
	if !isPidRunning(cmd.Process.Pid) {
		return fmt.Errorf("scheduler failed to start")
	}
	
	fmt.Printf("Scheduler started successfully with PID %d\n", cmd.Process.Pid)
	return nil
}


// startETLService starts the ETL service
func (sm *ServiceManager) startETLService(ctx context.Context) error {
	// First check if already running
	pid, err := sm.readPid(sm.config.ETLServicePIDFile)
	if err == nil && pid > 0 && isPidRunning(pid) {
		fmt.Println("ETL service is already running")
		return nil
	}

	// Path to the ETL directory and executable
	cliDir := filepath.Join(sm.config.QuotronRoot, "cli")
	etlDir := filepath.Join(cliDir, "cmd", "etl")
	etlExec := filepath.Join(etlDir, "etl")
	
	// Check if ETL executable exists, build if needed
	if _, err := os.Stat(etlExec); os.IsNotExist(err) || !isExecutable(etlExec) {
		fmt.Println("ETL executable not found or not executable. Building ETL...")
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "etl", "main.go")
		buildCmd.Dir = etlDir
		buildOut, err := buildCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to build ETL: %w, output: %s", err, buildOut)
		}
		fmt.Println("ETL built successfully")
	}
	
	// Get the platform-appropriate temp directory for logs
	logFile := sm.config.ETLServiceLogFile
	
	// Start the ETL service with appropriate parameters
	fmt.Println("Starting ETL service...")
	
	// Create command with all required arguments
	cmd := exec.CommandContext(ctx, etlExec,
		"-start",
		"-redis="+sm.config.RedisHost+":"+strconv.Itoa(sm.config.RedisPort),
		"-dbhost="+sm.config.DBHost,
		"-dbport="+strconv.Itoa(sm.config.DBPort),
		"-dbname="+sm.config.DBName,
		"-dbuser="+sm.config.DBUser,
		"-dbpass="+sm.config.DBPassword,
		"-workers=2",
	)
	
	// Set up log redirection
	logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open ETL log file: %w", err)
	}
	
	cmd.Stdout = logFd
	cmd.Stderr = logFd
	cmd.Dir = etlDir
	
	// Use nohup-like functionality to keep the process running
	// Create a process group independent of this process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
	
	// Start the ETL process
	err = cmd.Start()
	if err != nil {
		logFd.Close()
		return fmt.Errorf("failed to start ETL service: %w", err)
	}
	
	// Close the log file in the parent process - the child will keep it open
	logFd.Close()
	
	// Save PID to both the standard location and the legacy location
	err = sm.savePid(sm.config.ETLServicePIDFile, cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failed to save ETL service PID: %w", err)
	}
	
	// Also save PID to the legacy location for compatibility
	legacyPidPath := filepath.Join(cliDir, ".etl_service.pid")
	if err := os.WriteFile(legacyPidPath, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		fmt.Printf("Warning: Failed to write legacy PID file: %v\n", err)
	}
	
	// Add to global PID list
	addPid(cmd.Process.Pid)
	
	// Wait a moment to make sure it started properly
	time.Sleep(2 * time.Second)
	if isPidRunning(cmd.Process.Pid) {
		fmt.Printf("ETL service started successfully with PID %d\n", cmd.Process.Pid)
		fmt.Printf("Log file: %s\n", logFile)
	} else {
		return fmt.Errorf("ETL service failed to start. Check log at %s", logFile)
	}
	
	return nil
}

// stopService stops a service given its name and PID file
func (sm *ServiceManager) stopService(name, pidFile, processPattern string) error {
	fmt.Printf("Stopping %s...\n", name)
	
	// Try to stop using PID file first
	pid, err := sm.readPid(pidFile)
	if err == nil && pid > 0 {
		if isPidRunning(pid) {
			// Try to gracefully terminate the process
			process, err := os.FindProcess(pid)
			if err == nil {
				_ = process.Signal(syscall.SIGTERM)
				
				// Wait a bit for the process to terminate
				for i := 0; i < 5; i++ {
					time.Sleep(500 * time.Millisecond)
					if !isPidRunning(pid) {
						break
					}
				}
				
				// If process is still running, force kill it
				if isPidRunning(pid) {
					_ = process.Kill()
				}
			}
		}
		
		// Remove PID file regardless of whether process was stopped
		_ = os.Remove(pidFile)
	}
	
	// Double check if any processes matching the pattern are still running
	if processPattern != "" {
		cmd := exec.Command("pgrep", "-f", processPattern)
		output, _ := cmd.Output()
		if len(output) > 0 {
			// Find and kill all matching processes
			killCmd := exec.Command("pkill", "-9", "-f", processPattern)
			_ = killCmd.Run()
			fmt.Printf("%s stopped forcefully\n", name)
		} else {
			fmt.Printf("%s stopped successfully\n", name)
		}
	} else {
		fmt.Printf("%s stopped successfully\n", name)
	}
	
	return nil
}

// monitorServices monitors service health and restarts failed services
func (sm *ServiceManager) monitorServices(ctx context.Context, services ServiceList) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check service health
			status, err := sm.GetServiceStatus()
			if err != nil {
				fmt.Printf("Failed to check service status: %v\n", err)
				continue
			}
			
			// Restart failed services
			if services.YFinanceProxy && !status.YFinanceProxy {
				fmt.Println("YFinance Proxy is not running, restarting...")
				err := sm.startYFinanceProxy(ctx)
				if err != nil {
					fmt.Printf("Failed to restart YFinance Proxy: %v\n", err)
				}
			}
			
			if services.APIService && !status.APIService {
				fmt.Println("API Service is not running, restarting...")
				err := sm.startAPIService(ctx)
				if err != nil {
					fmt.Printf("Failed to restart API Service: %v\n", err)
				}
			}
			
			if services.Scheduler && !status.Scheduler {
				fmt.Println("Scheduler is not running, restarting...")
				err := sm.startScheduler(ctx)
				if err != nil {
					fmt.Printf("Failed to restart Scheduler: %v\n", err)
				}
			}
			
		}
	}
}

// Global list of PIDs to track for cleanup
var pidList []int

// addPid adds a PID to the global PID list
func addPid(pid int) {
	pidList = append(pidList, pid)
}

// clearPids clears the global PID list
func clearPids() {
	pidList = []int{}
}

// isPidRunning checks if a process is running
func isPidRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// isExecutable checks if a file is executable
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&0111 != 0
}

// savePid saves a process ID to a file
func (sm *ServiceManager) savePid(pidFile string, pid int) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(pidFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	
	// Write PID to file
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	
	return nil
}

// readPid reads a process ID from a file
func (sm *ServiceManager) readPid(pidFile string) (int, error) {
	// Check if file exists
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return 0, fmt.Errorf("PID file %s does not exist", pidFile)
	}
	
	// Read PID from file
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}
	
	// Parse PID
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("failed to parse PID: %w", err)
	}
	
	return pid, nil
}

// checkServiceRunning checks if a service is running using PID file and process pattern
func (sm *ServiceManager) checkServiceRunning(pidFile, processPattern string, host string, port int) bool {
	// First check if the service is responding on its port
	if host != "" && port > 0 {
		if sm.checkServiceResponding(host, port) {
			return true
		}
	}
	
	// Then check if a process with the PID in the PID file is running
	pid, err := sm.readPid(pidFile)
	if err == nil && pid > 0 && isPidRunning(pid) {
		return true
	}
	
	// Finally, check if any processes matching the pattern are running
	if processPattern != "" {
		cmd := exec.Command("pgrep", "-f", processPattern)
		if cmd.Run() == nil {
			return true
		}
	}
	
	return false
}

// checkServiceResponding checks if a service is responding on its port
func (sm *ServiceManager) checkServiceResponding(host string, port int) bool {
	if host == "" || port <= 0 {
		return false
	}
	
	// First check if the port is open
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 1*time.Second)
	if err != nil {
		fmt.Printf("Port %d is not open on %s: %v\n", port, host, err)
		return false
	}
	defer conn.Close()
	
	// Then check if the service has a root HTTP endpoint
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	
	url := fmt.Sprintf("http://%s:%d", host, port)
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	// Read response body up to 1KB to avoid memory issues with large responses
	buffer := make([]byte, 1024)
	_, _ = io.ReadAtLeast(resp.Body, buffer, 1)
	
	fmt.Printf("Service %s is responding at root URL (status: %d)\n", url, resp.StatusCode)
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

// GetConfig returns the current configuration
func (sm *ServiceManager) GetConfig() *Config {
	return sm.config
}

// readConfigJSON reads and parses config from JSON file
func readConfigJSON(configPath string) (map[string]interface{}, error) {
	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Parse JSON
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}
	
	return config, nil
}