package services

import (
	"context"
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
	Dashboard     bool
}

// ServiceStatus represents the running status of each service
type ServiceStatus struct {
	YFinanceProxy bool
	APIService    bool
	Scheduler     bool
	Dashboard     bool
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
	// Create cleanup function if not in monitor mode
	if !monitor {
		defer func() {
			for _, pid := range pidList {
				// Only kill processes we started in this session
				if pid > 0 {
					syscall.Kill(pid, syscall.SIGTERM)
				}
			}
		}()
	}

	// Build start order based on dependencies
	if services.APIService && !services.YFinanceProxy {
		// API service requires YFinance proxy
		services.YFinanceProxy = true
	}
	if services.Dashboard && !services.APIService {
		// Dashboard usually needs API service
		services.APIService = true
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

	if services.Dashboard {
		err := sm.startDashboard(ctx)
		if err != nil {
			return fmt.Errorf("failed to start Dashboard: %w", err)
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
	if services.Dashboard {
		err := sm.stopService("Dashboard", sm.config.DashboardPIDFile, "python.*dashboard.py")
		if err != nil {
			return fmt.Errorf("failed to stop Dashboard: %w", err)
		}
	}

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

	if services.YFinanceProxy {
		err := sm.stopService("YFinance Proxy", sm.config.YFinanceProxyPIDFile, "python.*yfinance_proxy.py")
		if err != nil {
			return fmt.Errorf("failed to stop YFinance Proxy: %w", err)
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
		Dashboard:     false,
	}

	// Check YFinance Proxy
	status.YFinanceProxy = sm.checkServiceRunning(sm.config.YFinanceProxyPIDFile, "python.*yfinance_proxy.py",
		sm.config.YFinanceProxyHost, sm.config.YFinanceProxyPort)

	// Check API Service
	status.APIService = sm.checkServiceRunning(sm.config.APIServicePIDFile, "api-service",
		sm.config.APIHost, sm.config.APIPort)

	// Check Scheduler
	status.Scheduler = sm.checkServiceRunning(sm.config.SchedulerPIDFile, "scheduler", "", 0)

	// Check Dashboard
	status.Dashboard = sm.checkServiceRunning(sm.config.DashboardPIDFile, "python.*dashboard.py",
		sm.config.DashboardHost, sm.config.DashboardPort)

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
	
	// Kill any existing process
	sm.stopService("YFinance Proxy", sm.config.YFinanceProxyPIDFile, "python.*yfinance_proxy.py")
	
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
	
	// Command with explicit host and port settings
	fmt.Println("Starting YFinance Proxy...")
	cmd := exec.CommandContext(ctx, pythonPath, scriptPath,
		"--host", sm.config.YFinanceProxyHost,
		"--port", strconv.Itoa(sm.config.YFinanceProxyPort))
	cmd.Dir = scriptsDir
	
	// Using direct logging rather than pipes
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	// Add important environment variables
	cmd.Env = append(os.Environ(), 
		"PYTHONUNBUFFERED=1",  // Force unbuffered output for better logs
		"LC_ALL=C.UTF-8",      // Ensure proper encoding
		fmt.Sprintf("HEALTH_SERVICE_URL=%s", sm.config.HealthServiceURL))
	
	// Prevent signal propagation
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	
	// Start it
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start YFinance Proxy: %w", err)
	}
	
	// Save PID and add to cleanup list
	pid := cmd.Process.Pid
	sm.savePid(sm.config.YFinanceProxyPIDFile, pid)
	addPid(pid)
	
	// Give it a moment to start
	fmt.Println("Waiting for YFinance Proxy to initialize...")
	time.Sleep(3 * time.Second)
	
	// Check if still running
	if !isPidRunning(pid) {
		logTail, _ := exec.Command("tail", "-n", "20", sm.config.YFinanceLogFile).Output()
		fmt.Printf("\nError: process terminated unexpectedly\n%s\n", string(logTail))
		return fmt.Errorf("YFinance Proxy process terminated immediately after starting")
	}
	
	// Wait for service to be responsive
	fmt.Printf("Waiting for YFinance Proxy to respond on %s:%d...\n", 
		sm.config.YFinanceProxyHost, sm.config.YFinanceProxyPort)
	
	// Extended wait time for slower machines
	err = sm.waitForService(sm.config.YFinanceProxyHost, sm.config.YFinanceProxyPort, 45*time.Second)
	if err != nil {
		fmt.Printf("WARNING: Service started (PID %d) but not responding on port %d.\n", 
			pid, sm.config.YFinanceProxyPort)
		fmt.Printf("The service may still be starting up. You can check the UI manually at http://%s:%d\n", 
			sm.config.YFinanceProxyHost, sm.config.YFinanceProxyPort)
		fmt.Printf("Or check logs at %s for errors.\n", sm.config.YFinanceLogFile)
		
		// Return success anyway, since we know the process is running
		return nil
	}
	
	fmt.Printf("YFinance Proxy started successfully with PID %d\n", pid)
	fmt.Printf("UI available at http://%s:%d\n", sm.config.YFinanceProxyHost, sm.config.YFinanceProxyPort)
	return nil
}

// startAPIService starts the API service
func (sm *ServiceManager) startAPIService(ctx context.Context) error {
	// Check if already running
	if sm.checkServiceRunning(sm.config.APIServicePIDFile, "api-service",
		sm.config.APIHost, sm.config.APIPort) {
		fmt.Println("API Service is already running")
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
	if err != nil || !isExecutable(apiServiceBin) {
		fmt.Println("Building API service...")

		// Ensure directories exist
		err := os.MkdirAll(filepath.Join(apiServiceDir, "cmd", "server"), 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		err = os.MkdirAll(filepath.Join(apiServiceDir, "pkg", "client"), 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Build the service
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "api-service", "./cmd/server")
		buildCmd.Dir = apiServiceDir
		buildOutput, err := buildCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to build API service: %w, output: %s", err, buildOutput)
		}
		fmt.Println("API service built successfully")
	}

	// Prepare command
	cmd := exec.CommandContext(ctx, apiServiceBin,
		"--port", strconv.Itoa(sm.config.APIPort),
		"--yahoo-host", sm.config.YFinanceProxyHost,
		"--yahoo-port", strconv.Itoa(sm.config.YFinanceProxyPort))

	// Set working directory
	cmd.Dir = apiServiceDir

	// Redirect output to log file
	logFile, err := os.OpenFile(sm.config.APIServiceLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start the process
	fmt.Println("Starting API service...")
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Save PID
	err = sm.savePid(sm.config.APIServicePIDFile, cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failed to save PID: %w", err)
	}

	// Add to global PID list for cleanup
	addPid(cmd.Process.Pid)

	// Wait for service to be available
	err = sm.waitForService(sm.config.APIHost, sm.config.APIPort, 30*time.Second)
	if err != nil {
		return fmt.Errorf("service failed to start: %w", err)
	}

	fmt.Printf("API service started successfully with PID %d\n", cmd.Process.Pid)
	return nil
}

// startScheduler starts the scheduler
func (sm *ServiceManager) startScheduler(ctx context.Context) error {
	// Check if already running
	if sm.checkServiceRunning(sm.config.SchedulerPIDFile, "scheduler", "", 0) {
		fmt.Println("Scheduler is already running")
		return nil
	}

	// Build scheduler if needed
	schedulerDir := sm.config.SchedulerPath
	schedulerBin := filepath.Join(schedulerDir, "scheduler")

	// Check if binary exists and is executable
	_, err := os.Stat(schedulerBin)
	if err != nil || !isExecutable(schedulerBin) {
		fmt.Println("Building Scheduler...")

		// Build the service
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "scheduler", "./cmd/scheduler/main.go")
		buildCmd.Dir = schedulerDir
		buildOutput, err := buildCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to build Scheduler: %w, output: %s", err, buildOutput)
		}
		fmt.Println("Scheduler built successfully")

		// Make executable
		err = os.Chmod(schedulerBin, 0755)
		if err != nil {
			return fmt.Errorf("failed to make scheduler executable: %w", err)
		}
	}

	// Check if API service is running to determine if we should use API service mode
	useAPIService := sm.checkServiceRunning("", "",
		sm.config.APIHost, sm.config.APIPort)

	// Ensure API scraper is built
	apiScraperBin := sm.config.APIScraperPath
	if !useAPIService {
		// Only need to check API scraper if not using API service
		_, err := os.Stat(apiScraperBin)
		if err != nil || !isExecutable(apiScraperBin) {
			fmt.Println("Building API scraper...")
			apiScraperDir := filepath.Join(sm.config.QuotronRoot, "api-scraper")
			buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "api-scraper", "./cmd/main/main.go")
			buildCmd.Dir = apiScraperDir
			buildOutput, err := buildCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to build API scraper: %w, output: %s", err, buildOutput)
			}
			fmt.Println("API scraper built successfully")

			// Make executable
			err = os.Chmod(apiScraperBin, 0755)
			if err != nil {
				return fmt.Errorf("failed to make API scraper executable: %w", err)
			}
		}
	}

	// Prepare command
	args := []string{}

	// Add config file if available
	configFile := filepath.Join(sm.config.QuotronRoot, "scheduler-config.json")
	if _, err := os.Stat(configFile); err == nil {
		args = append(args, "--config", configFile)
	}

	// Set API service mode if available
	if useAPIService {
		args = append(args, "--use-api-service",
			"--api-host", sm.config.APIHost,
			"--api-port", strconv.Itoa(sm.config.APIPort))
	} else {
		// Use API scraper directly
		args = append(args, "--api-scraper", apiScraperBin)
	}

	// Set API key if available
	if sm.config.AlphaVantageAPIKey != "" {
		os.Setenv("ALPHA_VANTAGE_API_KEY", sm.config.AlphaVantageAPIKey)
	}

	cmd := exec.CommandContext(ctx, schedulerBin, args...)

	// Set working directory
	cmd.Dir = schedulerDir

	// Redirect output to log file
	logFile, err := os.OpenFile(sm.config.SchedulerLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start the process
	fmt.Println("Starting Scheduler...")
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Save PID
	err = sm.savePid(sm.config.SchedulerPIDFile, cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failed to save PID: %w", err)
	}

	// Add to global PID list for cleanup
	addPid(cmd.Process.Pid)

	// Wait a bit to make sure scheduler started properly
	time.Sleep(3 * time.Second)

	// Check if process is still running
	if !isPidRunning(cmd.Process.Pid) {
		return fmt.Errorf("scheduler failed to start, check logs at %s", sm.config.SchedulerLogFile)
	}

	fmt.Printf("Scheduler started successfully with PID %d\n", cmd.Process.Pid)
	return nil
}

// startDashboard starts the dashboard
func (sm *ServiceManager) startDashboard(ctx context.Context) error {
	// Check if already running
	if sm.checkServiceRunning(sm.config.DashboardPIDFile, "python.*dashboard.py",
		sm.config.DashboardHost, sm.config.DashboardPort) {
		fmt.Println("Dashboard is already running")
		return nil
	}

	// Ensure directory exists
	dashboardDir := filepath.Join(sm.config.QuotronRoot, "dashboard")
	err := os.MkdirAll(dashboardDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Set environment variables
	env := os.Environ()
	env = append(env, fmt.Sprintf("YFINANCE_PROXY_URL=%s", sm.config.YFinanceProxyURL))
	env = append(env, fmt.Sprintf("API_SCRAPER_PATH=%s", sm.config.APIScraperPath))
	env = append(env, fmt.Sprintf("SCHEDULER_PATH=%s", sm.config.SchedulerPath))

	// Prepare command
	cmd := exec.CommandContext(ctx, "python", "dashboard.py")
	cmd.Dir = dashboardDir
	cmd.Env = env

	// Redirect output to log file
	logFile, err := os.OpenFile(sm.config.DashboardLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start the process
	fmt.Println("Starting Dashboard...")
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Save PID
	err = sm.savePid(sm.config.DashboardPIDFile, cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failed to save PID: %w", err)
	}

	// Add to global PID list for cleanup
	addPid(cmd.Process.Pid)

	// Wait a moment for Dashboard to start
	time.Sleep(5 * time.Second)

	// Check if process is still running
	if !isPidRunning(cmd.Process.Pid) {
		return fmt.Errorf("dashboard failed to start, check logs at %s", sm.config.DashboardLogFile)
	}

	// Wait for service to be available
	err = sm.waitForService(sm.config.DashboardHost, sm.config.DashboardPort, 30*time.Second)
	if err != nil {
		fmt.Printf("Warning: Dashboard process is running, but port %d is not responding yet.\n", sm.config.DashboardPort)
		fmt.Println("It may still be initializing. Check status later or view the logs.")
	} else {
		fmt.Printf("Dashboard started successfully with PID %d\n", cmd.Process.Pid)
		fmt.Printf("Dashboard available at http://%s:%d\n", sm.config.DashboardHost, sm.config.DashboardPort)
	}

	return nil
}

// stopService stops a service by name and pattern
func (sm *ServiceManager) stopService(name, pidFile, pattern string) error {
	fmt.Printf("Stopping %s...\n", name)

	// Try to stop using PID file first
	if pidFile != "" {
		pid, err := sm.readPid(pidFile)
		if err == nil && pid > 0 {
			// Try to terminate gracefully
			err = syscall.Kill(pid, syscall.SIGTERM)
			if err == nil {
				// Wait for process to terminate
				for i := 0; i < 5; i++ {
					if !isPidRunning(pid) {
						fmt.Printf("%s stopped successfully\n", name)
						os.Remove(pidFile)
						return nil
					}
					time.Sleep(1 * time.Second)
				}

				// Force kill if still running
				fmt.Printf("%s is not responding. Sending SIGKILL...\n", name)
				err = syscall.Kill(pid, syscall.SIGKILL)
				if err == nil {
					os.Remove(pidFile)
					return nil
				}
			}
		}
	}

	// If PID file approach failed, try using pattern matching
	if pattern != "" {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("pgrep -f '%s'", pattern))
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			// Get the PIDs
			pidsStr := strings.TrimSpace(string(output))
			pidStrings := strings.Split(pidsStr, "\n")

			for _, pidStr := range pidStrings {
				pid, err := strconv.Atoi(pidStr)
				if err != nil {
					continue
				}

				// Try to terminate gracefully
				syscall.Kill(pid, syscall.SIGTERM)
			}

			// Wait a moment
			time.Sleep(2 * time.Second)

			// Check if processes are still running
			cmd = exec.Command("bash", "-c", fmt.Sprintf("pgrep -f '%s'", pattern))
			output, err = cmd.Output()
			if err != nil || len(output) == 0 {
				fmt.Printf("%s stopped successfully\n", name)
				if pidFile != "" {
					os.Remove(pidFile)
				}
				return nil
			}

			// Force kill if still running
			fmt.Printf("%s is not responding. Sending SIGKILL...\n", name)
			exec.Command("bash", "-c", fmt.Sprintf("pkill -9 -f '%s'", pattern)).Run()
			if pidFile != "" {
				os.Remove(pidFile)
			}
			return nil
		}
	}

	fmt.Printf("%s is not running\n", name)
	if pidFile != "" {
		os.Remove(pidFile)
	}
	return nil
}

// checkServiceRunning checks if a service is running
func (sm *ServiceManager) checkServiceRunning(pidFile, pattern string, host string, port int) bool {
	// Check by PID file first
	if pidFile != "" {
		pid, err := sm.readPid(pidFile)
		if err == nil && pid > 0 {
			if isPidRunning(pid) {
				// If host and port are provided, check if service is responding
				if host != "" && port > 0 {
					if sm.checkServiceResponding(host, port) {
						return true
					}
				} else {
					return true
				}
			}
		}
	}

	// Check by pattern if provided
	if pattern != "" {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("pgrep -f '%s'", pattern))
		if err := cmd.Run(); err == nil {
			// If host and port are provided, check if service is responding
			if host != "" && port > 0 {
				return sm.checkServiceResponding(host, port)
			}
			return true
		}
	}

	// If no PID file or pattern, just check if service is responding
	if host != "" && port > 0 {
		return sm.checkServiceResponding(host, port)
	}

	return false
}

// checkServiceResponding checks if a service is responding on the given host and port
func (sm *ServiceManager) checkServiceResponding(host string, port int) bool {
	// Try both health and root endpoints with a longer timeout
	client := &http.Client{
		Timeout: 5 * time.Second, // Increased timeout to 5 seconds
	}
	
	// Check if the port is open first (TCP check)
	tcpAddr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", tcpAddr, 2*time.Second)
	if err != nil {
		fmt.Printf("Port %d is not open on %s: %v\n", port, host, err)
		return false
	}
	conn.Close()
	
	// First try the root URL (for our new UI)
	rootURL := fmt.Sprintf("http://%s:%d", host, port)
	req, _ := http.NewRequest("GET", rootURL, nil)
	
	// Set important headers
	req.Header.Set("Accept", "text/html, */*")
	req.Header.Set("User-Agent", "CLI-HealthChecker/1.0")
	
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		
		// Read a small part of the response
		buf := make([]byte, 1024)
		_, err := resp.Body.Read(buf)
		if err != nil && err != io.EOF {
			fmt.Printf("Error reading response body: %v\n", err)
		} else {
			fmt.Printf("Service %s:%d is responding at root URL (status: %d)\n", 
				host, port, resp.StatusCode)
			return true
		}
	}
	
	// Then try the health endpoint
	healthURL := fmt.Sprintf("http://%s:%d/health", host, port)
	req, _ = http.NewRequest("GET", healthURL, nil)
	req.Header.Set("Accept", "application/json, */*")
	req.Header.Set("User-Agent", "CLI-HealthChecker/1.0")
	
	resp, err = client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		fmt.Printf("Service %s:%d is responding at health endpoint (status: %d)\n", 
			host, port, resp.StatusCode)
		return true
	}
	
	fmt.Printf("Service %s:%d is not responding properly (TCP connection succeeded but HTTP failed)\n", 
		host, port)
	return false
}

// waitForService waits for a service to be available
func (sm *ServiceManager) waitForService(host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	attempts := 0
	startTime := time.Now()
	
	fmt.Printf("Waiting for service at %s:%d to respond (timeout: %s)...\n", 
		host, port, timeout)
	
	for time.Now().Before(deadline) {
		attempts++
		if sm.checkServiceResponding(host, port) {
			elapsed := time.Since(startTime).Seconds()
			fmt.Printf("Service available after %.1f seconds (%d attempts)\n", elapsed, attempts)
			return nil
		}
		
		// Check if the process has died while waiting
		if attempts%5 == 0 { // Check every 5 seconds
			// For now we'll just display a waiting message
			fmt.Printf("Still waiting for service to respond (%.1f seconds elapsed, %d attempts)...\n", 
				time.Since(startTime).Seconds(), attempts)
		}
		
		time.Sleep(2 * time.Second) // Increased sleep time between attempts
	}
	
	// Timeout occurred
	return fmt.Errorf("service not available after %s (%d connection attempts) - try running it manually with 'cd /home/hunter/Desktop/tiny-ria/quotron/api-scraper/scripts && python3 yfinance_proxy.py'", 
		timeout, attempts)
}

// savePid saves a PID to a file
func (sm *ServiceManager) savePid(pidFile string, pid int) error {
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// readPid reads a PID from a file
func (sm *ServiceManager) readPid(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// monitorServices monitors services and restarts them if they fail
func (sm *ServiceManager) monitorServices(ctx context.Context, services ServiceList) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status, err := sm.GetServiceStatus()
			if err != nil {
				fmt.Printf("Error checking service status: %v\n", err)
				continue
			}

			// Check each service
			if services.YFinanceProxy && !status.YFinanceProxy {
				fmt.Println("YFinance Proxy is down, restarting...")
				err := sm.startYFinanceProxy(ctx)
				if err != nil {
					fmt.Printf("Failed to restart YFinance Proxy: %v\n", err)
				}
			}

			if services.APIService && !status.APIService {
				fmt.Println("API Service is down, restarting...")
				err := sm.startAPIService(ctx)
				if err != nil {
					fmt.Printf("Failed to restart API Service: %v\n", err)
				}
			}

			if services.Scheduler && !status.Scheduler {
				fmt.Println("Scheduler is down, restarting...")
				err := sm.startScheduler(ctx)
				if err != nil {
					fmt.Printf("Failed to restart Scheduler: %v\n", err)
				}
			}

			if services.Dashboard && !status.Dashboard {
				fmt.Println("Dashboard is down, restarting...")
				err := sm.startDashboard(ctx)
				if err != nil {
					fmt.Printf("Failed to restart Dashboard: %v\n", err)
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