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
	Dashboard     bool
	ETLService    bool
}

// ServiceStatus represents the running status of each service
type ServiceStatus struct {
	YFinanceProxy bool
	APIService    bool
	Scheduler     bool
	Dashboard     bool
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
		Dashboard:     false,
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

	// Check Dashboard
	status.Dashboard = sm.checkServiceRunning(sm.config.DashboardPIDFile, "python.*dashboard.py",
		sm.config.DashboardHost, sm.config.DashboardPort)
		
	// Check ETL Service
	status.ETLService = sm.checkServiceRunning(sm.config.ETLServicePIDFile, "etl.*-start", "", 0)

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
	if err != nil || !isExecutable(apiServiceBin) {
		fmt.Println("Building API service...")

		// Build the service with main package
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "api-service", "./cmd/main")
		buildCmd.Dir = apiServiceDir
		buildOutput, err := buildCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to build API service: %w, output: %s", err, buildOutput)
		}
		fmt.Println("API service built successfully")
	}

	// Create a script to run the service with nohup
	scriptContent := `#!/bin/bash
cd "$(dirname "$0")"
nohup ./api-service \
  --port=%d \
  --yahoo-host=%s \
  --yahoo-port=%d \
  --health=false \
  >> %s 2>&1 &
echo $! > %s
`
	scriptPath := filepath.Join(apiServiceDir, "run_api.sh")
	scriptContent = fmt.Sprintf(scriptContent, 
		sm.config.APIPort, 
		sm.config.YFinanceProxyHost, 
		sm.config.YFinanceProxyPort,
		sm.config.APIServiceLogFile,
		sm.config.APIServicePIDFile)
	
	err = os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		return fmt.Errorf("failed to create script: %w", err)
	}

	// Start the process using the script
	fmt.Println("Starting API service...")
	cmd := exec.CommandContext(context.Background(), scriptPath)
	cmd.Dir = apiServiceDir
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Wait for service to be available
	err = sm.waitForService(sm.config.APIHost, sm.config.APIPort, 30*time.Second)
	if err != nil {
		return fmt.Errorf("service failed to start: %w", err)
	}

	// Read the PID from the file
	pid, err := sm.readPid(sm.config.APIServicePIDFile)
	if err != nil {
		fmt.Printf("Warning: Could not read PID file: %v\n", err)
		// Not fatal, continue
	} else {
		fmt.Printf("API service started successfully with PID %d\n", pid)
	}
	
	return nil
}

// startScheduler starts the scheduler directly without shell scripts
func (sm *ServiceManager) startScheduler(ctx context.Context) error {
	// First check if scheduler is already running
	cmd := exec.Command("pgrep", "-f", "scheduler --config")
	if err := cmd.Run(); err == nil {
		fmt.Println("Scheduler is already running")
		return nil
	}
	
	// Kill any existing processes to be sure
	exec.Command("pkill", "-f", "scheduler").Run()
	
	// Remove any stale PID files
	os.Remove(sm.config.SchedulerPIDFile)
	
	// Build scheduler if needed
	schedulerDir := sm.config.SchedulerPath
	schedulerBin := filepath.Join(schedulerDir, "scheduler")
	configFile := filepath.Join(sm.config.QuotronRoot, "scheduler-config.json")

	// Check if binary exists and is executable
	_, err := os.Stat(schedulerBin)
	if err != nil || !isExecutable(schedulerBin) {
		fmt.Println("Building Scheduler...")

		// Build the service
		buildCmd := exec.Command("go", "build", "-o", "scheduler", "./cmd/scheduler/main.go")
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

	// Start the scheduler directly
	fmt.Println("Starting Scheduler...")
	
	// Create a command that uses nohup
	runCmd := exec.Command("nohup", schedulerBin, "--config", configFile)
	runCmd.Dir = schedulerDir
	
	// Set up log redirection
	logFile, err := os.OpenFile(sm.config.SchedulerLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()
	
	runCmd.Stdout = logFile
	runCmd.Stderr = logFile
	
	// Start the process
	if err := runCmd.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}
	
	// Save PID
	pid := runCmd.Process.Pid
	err = sm.savePid(sm.config.SchedulerPIDFile, pid)
	if err != nil {
		fmt.Printf("Warning: Failed to save PID file: %v\n", err)
		// Not fatal
	}
	
	// Detach process (prevent Go from waiting for it)
	runCmd.Process.Release()

	fmt.Printf("Scheduler started successfully with PID %d\n", pid)
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
	// Check if a process with a specific pattern is running
	isProcessRunning := func(pat string) bool {
		if pat == "" {
			return false
		}
		cmd := exec.Command("bash", "-c", fmt.Sprintf("pgrep -f '%s'", pat))
		return cmd.Run() == nil
	}
	
	// Check by PID file first
	if pidFile != "" {
		pid, err := sm.readPid(pidFile)
		if err == nil && pid > 0 {
			if isPidRunning(pid) {
				// If host and port are provided, also check if service is responding
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
	if pattern != "" && isProcessRunning(pattern) {
		// If host and port are provided, also check if service is responding
		if host != "" && port > 0 {
			return sm.checkServiceResponding(host, port)
		}
		return true
	}

	// If no PID file or pattern, just check if service is responding
	if host != "" && port > 0 {
		return sm.checkServiceResponding(host, port)
	}

	return false
}

// ListSchedulerJobs returns a list of jobs in the scheduler
func (sm *ServiceManager) ListSchedulerJobs() ([]map[string]interface{}, error) {
	// Check if scheduler is running
	if !sm.checkServiceRunning(sm.config.SchedulerPIDFile, "scheduler", "", 0) {
		return nil, fmt.Errorf("scheduler is not running")
	}

	// Get the config file path
	configFile := filepath.Join(sm.config.QuotronRoot, "scheduler-config.json")
	
	// Read the config file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read scheduler config: %w", err)
	}
	
	// Parse the config file to get job information
	var config struct {
		Schedules map[string]map[string]interface{} `json:"schedules"`
	}
	
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse scheduler config: %w", err)
	}
	
	// Convert to a list of jobs
	jobs := make([]map[string]interface{}, 0, len(config.Schedules))
	for name, job := range config.Schedules {
		jobInfo := map[string]interface{}{
			"name": name,
		}
		
		// Copy over job properties
		for k, v := range job {
			jobInfo[k] = v
		}
		
		jobs = append(jobs, jobInfo)
	}
	
	return jobs, nil
}

// RunSchedulerJob runs a specific scheduler job
func (sm *ServiceManager) RunSchedulerJob(jobName string) error {
	// Check if scheduler is running
	if !sm.checkServiceRunning(sm.config.SchedulerPIDFile, "scheduler", "", 0) {
		return fmt.Errorf("scheduler is not running")
	}
	
	// Run the job using the scheduler binary
	fmt.Printf("Running scheduler job '%s'...\n", jobName)
	schedulerDir := sm.config.SchedulerPath
	schedulerBin := filepath.Join(schedulerDir, "scheduler")
	configFile := filepath.Join(sm.config.QuotronRoot, "scheduler-config.json")
	
	// Build the command
	cmd := exec.Command(schedulerBin, "--config", configFile, "--run-job", jobName)
	cmd.Dir = schedulerDir
	
	// Set up output redirection
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Run the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run job '%s': %w", jobName, err)
	}
	
	fmt.Printf("Job '%s' executed successfully\n", jobName)
	return nil
}

// GetSchedulerNextRuns returns the next run time for each job
func (sm *ServiceManager) GetSchedulerNextRuns() (map[string]string, error) {
	// Check if scheduler is running
	pid, err := sm.readPid(sm.config.SchedulerPIDFile)
	if err != nil || !isPidRunning(pid) {
		return nil, fmt.Errorf("scheduler is not running")
	}
	
	// Look for scheduler log entries showing next run times
	logFile := sm.config.SchedulerLogFile
	
	// Read the log file
	logData, err := os.ReadFile(logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read scheduler log: %w", err)
	}
	
	// Parse the log to find next run times
	logLines := strings.Split(string(logData), "\n")
	nextRuns := make(map[string]string)
	
	// Look for lines like: "- stock_quotes: Next run at 2025-03-16T09:00:00Z"
	for _, line := range logLines {
		if strings.Contains(line, "Next run at") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				jobName := strings.TrimSpace(strings.TrimPrefix(parts[0], "-"))
				timeStr := strings.TrimSpace(strings.Join(parts[2:], ":"))
				nextRuns[jobName] = timeStr
			}
		}
	}
	
	return nextRuns, nil
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
	scriptsDir := filepath.Join(sm.config.QuotronRoot, "api-scraper", "scripts")
	return fmt.Errorf("service not available after %s (%d connection attempts) - try running it manually with 'cd %s && python3 yfinance_proxy.py'", 
		timeout, attempts, scriptsDir)
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

// startETLService starts the ETL service
func (sm *ServiceManager) startETLService(ctx context.Context) error {
	// Check if already running
	if sm.checkServiceRunning(sm.config.ETLServicePIDFile, "etl.*-start", "", 0) {
		fmt.Println("ETL Service is already running")
		return nil
	}
	
	// Build the ETL service if needed
	etlDir := filepath.Join(sm.config.QuotronRoot, "cli")
	etlBin := filepath.Join(etlDir, "cmd", "etl", "etl")
	
	// Check if the binary directory exists
	etlBinDir := filepath.Join(etlDir, "cmd", "etl")
	if _, err := os.Stat(etlBinDir); os.IsNotExist(err) {
		// Create the directory if it doesn't exist
		if err := os.MkdirAll(etlBinDir, 0755); err != nil {
			return fmt.Errorf("failed to create ETL binary directory: %w", err)
		}
	}
	
	// Check if binary exists and is executable
	_, err := os.Stat(etlBin)
	if err != nil || !isExecutable(etlBin) {
		fmt.Println("Building ETL service...")
		
		// Build the service
		buildCmd := exec.Command("go", "build", "-o", etlBin, "./cmd/etl")
		buildCmd.Dir = etlDir
		buildOutput, err := buildCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to build ETL service: %w, output: %s", err, buildOutput)
		}
		fmt.Println("ETL service built successfully")
		
		// Make executable
		err = os.Chmod(etlBin, 0755)
		if err != nil {
			return fmt.Errorf("failed to make ETL service executable: %w", err)
		}
	}
	
	// Prepare Redis connection string
	redisAddr := fmt.Sprintf("%s:%d", sm.config.RedisHost, sm.config.RedisPort)
	
	// Construct database connection string
//	dbConnStr := fmt.Sprintf(
//		"host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
//		sm.config.DBHost, sm.config.DBPort, sm.config.DBName, sm.config.DBUser, sm.config.DBPassword,
//	)
	
	// Start the ETL service
	fmt.Println("Starting ETL service...")
	
	// Create a script to run the ETL service with nohup
	scriptContent := `#!/bin/bash
cd "$(dirname "$0")"
nohup %s -start -redis=%s -dbhost=%s -dbport=%d -dbname=%s -dbuser=%s -dbpass=%s -workers=2 >> %s 2>&1 &
echo $! > %s
`
	scriptPath := filepath.Join(etlDir, "start_etl.sh")
	scriptContent = fmt.Sprintf(scriptContent,
		etlBin,
		redisAddr,
		sm.config.DBHost,
		sm.config.DBPort,
		sm.config.DBName,
		sm.config.DBUser,
		sm.config.DBPassword,
		sm.config.ETLServiceLogFile,
		sm.config.ETLServicePIDFile,
	)
	
	err = os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		return fmt.Errorf("failed to create ETL start script: %w", err)
	}
	
	// Execute the script
	cmd := exec.CommandContext(context.Background(), scriptPath)
	cmd.Dir = etlDir
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start ETL service: %w", err)
	}
	
	// Read the PID from the file
	time.Sleep(1 * time.Second) // Give it a moment to write the PID file
	pid, err := sm.readPid(sm.config.ETLServicePIDFile)
	if err != nil {
		fmt.Printf("Warning: Could not read ETL service PID file: %v\n", err)
	} else {
		fmt.Printf("ETL service started successfully with PID %d\n", pid)
	}
	
	// Check if process is running
	if !isPidRunning(pid) {
		return fmt.Errorf("ETL service failed to start, check logs at %s", sm.config.ETLServiceLogFile)
	}
	
	return nil
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