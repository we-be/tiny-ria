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
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/we-be/tiny-ria/quotron/cli/pkg/etl"
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
	redis  *redis.Client
}

// NewServiceManager creates a new ServiceManager
func NewServiceManager(config *Config) *ServiceManager {
	// Connect to Redis if available
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.RedisHost, config.RedisPort),
		Password: config.RedisPassword,
		DB:       0,
	})
	
	// Test connection - silently continue if Redis is not available
	_, err := redisClient.Ping(context.Background()).Result()
	if err != nil {
		fmt.Printf("Warning: Redis not available at %s:%d: %v\n", 
			config.RedisHost, config.RedisPort, err)
		redisClient = nil
	}
	
	return &ServiceManager{
		config: config,
		redis:  redisClient,
	}
}

// GetQuotronRoot returns the path to the quotron root directory
func (sm *ServiceManager) GetQuotronRoot() string {
	return sm.config.QuotronRoot
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
		// First try to stop the in-process ETL service
		etlServiceMutex.Lock()
		if etlService != nil && etlService.IsRunning() {
			fmt.Println("Stopping in-process ETL service...")
			etlService.Stop()
			etlService = nil
			if etlServiceCancel != nil {
				etlServiceCancel()
				etlServiceCancel = nil
			}
			
			// Update Redis status
			if sm.redis != nil {
				ctx := context.Background()
				err := sm.redis.HSet(ctx, "quotron:services:etl", map[string]interface{}{
					"status":    "stopped",
					"timestamp": time.Now().Unix(),
				}).Err()
				
				if err != nil {
					fmt.Printf("Warning: Failed to update ETL service status in Redis: %v\n", err)
				}
			}
			
			// Remove PID file
			if err := os.Remove(sm.config.ETLServicePIDFile); err != nil && !os.IsNotExist(err) {
				fmt.Printf("Warning: Failed to remove ETL PID file: %v\n", err)
			}
			
			fmt.Println("ETL service stopped successfully")
			etlServiceMutex.Unlock()
		} else {
			etlServiceMutex.Unlock()
			// Fall back to traditional process kill for backward compatibility
			// Update Redis status even for external process
			if sm.redis != nil {
				ctx := context.Background()
				sm.redis.HSet(ctx, "quotron:services:etl", map[string]interface{}{
					"status":    "stopped",
					"timestamp": time.Now().Unix(),
				})
			}
			
			err := sm.stopService("ETL Service", sm.config.ETLServicePIDFile, "etl.*-start")
			if err != nil {
				return fmt.Errorf("failed to stop ETL Service: %w", err)
			}
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
	etlServiceMutex.RLock()
	if etlService != nil && etlService.IsRunning() {
		status.ETLService = true
		etlServiceMutex.RUnlock()
	} else {
		etlServiceMutex.RUnlock()
		
		// Check Redis first if available
		if sm.redis != nil {
			ctx := context.Background()
			serviceInfo, err := sm.redis.HGetAll(ctx, "quotron:services:etl").Result()
			if err == nil && len(serviceInfo) > 0 {
				fmt.Printf("Debug: Redis service status: %v\n", serviceInfo)
				
				// Check if the service is reporting as running
				if serviceInfo["status"] == "running" {
					// Check if the PID is still running
					pidStr, ok := serviceInfo["pid"]
					if ok {
						pid, err := strconv.Atoi(pidStr)
						if err == nil && pid > 0 {
							fmt.Printf("Debug: Checking PID %d from Redis...\n", pid)
							if isPidRunning(pid) {
								fmt.Printf("Debug: PID %d is running\n", pid)
								// Check if timestamp is recent (within last 5 minutes)
								tsStr, tsOk := serviceInfo["timestamp"]
								if !tsOk {
									// No timestamp, assume it's running
									fmt.Println("Debug: No timestamp, assuming service is running")
									status.ETLService = true
								} else {
									ts, err := strconv.ParseInt(tsStr, 10, 64)
									if err == nil {
										lastSeen := time.Unix(ts, 0)
										timeSince := time.Since(lastSeen)
										fmt.Printf("Debug: Last update was %s ago\n", timeSince)
										if timeSince < 5*time.Minute {
											fmt.Println("Debug: Update is recent, service is running")
											status.ETLService = true
										}
									}
								}
							} else {
								fmt.Printf("Debug: PID %d is NOT running\n", pid)
							}
						}
					}
				}
			}
		}
		
		// Fall back to PID file if service not found in Redis or Redis unavailable
		if !status.ETLService {
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


// startAPIService starts the API service using Go runtime
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

	// Import API package
	apiPkg, err := sm.importAPIPackage()
	if err != nil {
		return fmt.Errorf("failed to import API package: %w", err)
	}

	// Start the API service
	fmt.Println("Starting API service...")
	
	// Ensure config dir exists
	configDir := filepath.Dir(sm.config.APIServicePIDFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Create API configuration
	config := apiPkg.CreateConfig(
		sm.config.APIPort,
		"postgres://postgres:postgres@localhost:5432/quotron?sslmode=disable", // Use config for this
		true, // useYahoo
		"", // alphaKey
		sm.config.YFinanceProxyHost,
		sm.config.YFinanceProxyPort,
		sm.config.HealthServiceURL != "",
		sm.config.HealthServiceURL,
		"api-service",
	)
	
	// Create a channel to receive errors from the API service
	errChan := make(chan error, 1)
	
	// Create a stop channel to pass to the API service
	stopChan := make(chan struct{})
	
	// Start the API service in a separate goroutine
	go func() {
		// Save PID
		pid := os.Getpid()
		err := sm.savePid(sm.config.APIServicePIDFile, pid)
		if err != nil {
			errChan <- fmt.Errorf("failed to save API Service PID: %w", err)
			return
		}
		
		// Add to global PID list
		addPid(pid)
		
		// Run the API service
		err = apiPkg.RunAPIService(config, stopChan)
		if err != nil {
			errChan <- fmt.Errorf("API service exited with error: %w", err)
		}
	}()
	
	// Wait for service to be responsive
	fmt.Printf("Waiting for service at %s:%d to respond (timeout: %ds)...\n", 
		sm.config.APIHost, sm.config.APIPort, 30)
	
	// Poll until service is responsive or timeout reached
	timeout := 30 * time.Second
	start := time.Now()
	attempts := 0
	
	// Use a ticker for polling
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	timeoutChan := time.After(timeout)
	
	for {
		select {
		case <-ctx.Done():
			// Context was cancelled
			close(stopChan) // Signal API service to shut down
			return fmt.Errorf("context cancelled while waiting for API service to start")
			
		case err := <-errChan:
			// API service encountered an error
			close(stopChan) // Ensure API service is shut down
			return err
			
		case <-timeoutChan:
			// Timeout reached
			close(stopChan) // Signal API service to shut down
			return fmt.Errorf("timed out waiting for API Service to respond")
			
		case <-ticker.C:
			// Check if service is responding
			attempts++
			if sm.checkServiceResponding(sm.config.APIHost, sm.config.APIPort) {
				fmt.Printf("Service available after %.1f seconds (%d attempts)\n", 
					time.Since(start).Seconds(), attempts)
				return nil
			}
			
			if attempts%5 == 0 {
				fmt.Printf("Still waiting for service to respond (%.1f seconds elapsed, %d attempts)...\n", 
					time.Since(start).Seconds(), attempts)
			}
		}
	}
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
// ETL service instance - global to ensure single instance
var etlService *etl.Service
var etlServiceCancel context.CancelFunc
var etlServiceMutex sync.RWMutex

func (sm *ServiceManager) startETLService(ctx context.Context) error {
	etlServiceMutex.Lock()
	defer etlServiceMutex.Unlock()

	// First check if our in-process service is already running
	if etlService != nil && etlService.IsRunning() {
		fmt.Println("ETL service is already running (in-process)")
		return nil
	}

	// For backward compatibility: check if another instance is running via PID file
	pid, err := sm.readPid(sm.config.ETLServicePIDFile)
	if err == nil && pid > 0 && isPidRunning(pid) {
		fmt.Println("ETL service is already running (external process)")
		return nil
	}

	// Configure database connection string
	dbConnStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		sm.config.DBHost, sm.config.DBPort, sm.config.DBUser, sm.config.DBPassword, sm.config.DBName,
	)

	// Configure Redis connection
	redisAddr := fmt.Sprintf("%s:%d", sm.config.RedisHost, sm.config.RedisPort)

	// Start the ETL service using the package directly
	fmt.Println("Starting ETL service (in-process)...")
	
	// Import the etl package
	etlPkg, err := sm.importETLPackage()
	if err != nil {
		return fmt.Errorf("failed to import ETL package: %w", err)
	}
	
	// Create a new ETL service instance
	etlService = etlPkg.NewService(redisAddr, dbConnStr, 2) // Use 2 workers
	
	// Start the service
	err = etlService.Start()
	if err != nil {
		return fmt.Errorf("failed to start ETL service: %w", err)
	}
	
	// Save service status to Redis
	if sm.redis != nil {
		ctx := context.Background()
		// Store service info in Redis hash
		err = sm.redis.HSet(ctx, "quotron:services:etl", map[string]interface{}{
			"pid":       os.Getpid(),
			"status":    "running",
			"host":      getHostname(),
			"timestamp": time.Now().Unix(),
		}).Err()
		
		if err != nil {
			fmt.Printf("Warning: Failed to store ETL service status in Redis: %v\n", err)
		} else {
			fmt.Println("Service status stored in Redis")
		}
	} else {
		// Fall back to PID file if Redis isn't available
		err = sm.savePid(sm.config.ETLServicePIDFile, os.Getpid())
		if err != nil {
			fmt.Printf("Warning: Failed to write PID file: %v\n", err)
		}
	}
	
	// Create a goroutine to monitor the service for cancellation
	etlCtx, cancel := context.WithCancel(context.Background())
	etlServiceCancel = cancel
	
	go func() {
		<-etlCtx.Done()
		etlServiceMutex.Lock()
		defer etlServiceMutex.Unlock()
		
		if etlService != nil && etlService.IsRunning() {
			fmt.Println("Stopping ETL service...")
			etlService.Stop()
			
			// Update Redis status on graceful shutdown
			if sm.redis != nil {
				ctx := context.Background()
				err := sm.redis.HSet(ctx, "quotron:services:etl", map[string]interface{}{
					"status":    "stopped",
					"timestamp": time.Now().Unix(),
				}).Err()
				
				if err != nil {
					fmt.Printf("Warning: Failed to update ETL service status in Redis: %v\n", err)
				}
			}
		}
	}()
	
	fmt.Printf("ETL service started successfully\n")
	return nil
}

// stopService stops a service given its name and PID file
func (sm *ServiceManager) stopService(name, pidFile, processPattern string) error {
	fmt.Printf("Stopping %s...\n", name)
	
	// Special handling for API service if it's running in the same process
	if name == "API Service" && os.Getpid() == sm.readSelfPid(pidFile) {
		fmt.Println("API Service is running in the same process, special shutdown required")
		// Send a signal to the API service's stop channel
		// This would require some global coordination, which we're not implementing yet
		// For now, we'll just continue with normal process termination
	}
	
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

// readSelfPid checks if the PID in the file matches the current process
func (sm *ServiceManager) readSelfPid(pidFile string) int {
	pid, err := sm.readPid(pidFile)
	if err != nil {
		return -1
	}
	return pid
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
			
			if services.ETLService && !status.ETLService {
				fmt.Println("ETL Service is not running, restarting...")
				err := sm.startETLService(ctx)
				if err != nil {
					fmt.Printf("Failed to restart ETL Service: %v\n", err)
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

// getHostname returns the hostname of the current machine
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
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

// RunSchedulerJob runs a scheduler job directly by loading the scheduler package
// and calling RunJobNow on it, without using the CLI
func (sm *ServiceManager) RunSchedulerJob(ctx context.Context, jobName string) error {
	// Import the scheduler package and configuration
	schedulerPkg, err := sm.importSchedulerPackage()
	if err != nil {
		return fmt.Errorf("failed to import scheduler package: %w", err)
	}

	// Run the job through the imported package
	err = schedulerPkg.RunJob(ctx, jobName, sm.config.SchedulerConfigFile)
	if err != nil {
		return fmt.Errorf("failed to run job %s: %w", jobName, err)
	}

	return nil
}

// importSchedulerPackage dynamically loads the scheduler package
// This is a helper function to avoid direct imports and allow for runtime loading
func (sm *ServiceManager) importSchedulerPackage() (*SchedulerPackage, error) {
	// Create a wrapper around the scheduler package
	return &SchedulerPackage{
		schedulerDir: filepath.Join(sm.config.QuotronRoot, "scheduler"),
	}, nil
}

// importAPIPackage dynamically loads the API package
// This is a helper function to avoid direct imports and allow for runtime loading
func (sm *ServiceManager) importAPIPackage() (*APIPackage, error) {
	// Create a wrapper around the API package
	return &APIPackage{
		apiServiceDir: filepath.Join(sm.config.QuotronRoot, "api-service"),
	}, nil
}

// importETLPackage loads the ETL package
// Unlike the other import functions, this actually imports the package directly
func (sm *ServiceManager) importETLPackage() (*etl.ETLPackage, error) {
	// Import the etl package directly
	return &etl.ETLPackage{}, nil
}

// SchedulerPackage is a wrapper around the scheduler package
// This allows us to access the scheduler package without direct imports
type SchedulerPackage struct {
	schedulerDir string
}

// APIPackage is a wrapper around the API package
// This allows us to access the API package without direct imports
type APIPackage struct {
	apiServiceDir string
}

// CreateConfig creates a configuration for the API service
func (ap *APIPackage) CreateConfig(
	port int,
	dbURL string,
	useYahoo bool,
	alphaKey string,
	yahooHost string,
	yahooPort int,
	useHealth bool,
	healthService string,
	serviceName string,
) interface{} {
	// Create a map to represent the Config struct
	return map[string]interface{}{
		"Port":           port,
		"DatabaseURL":    dbURL,
		"YahooEnabled":   useYahoo,
		"AlphaKey":       alphaKey,
		"YahooHost":      yahooHost,
		"YahooPort":      yahooPort,
		"HealthEnabled":  useHealth,
		"HealthService":  healthService,
		"ServiceName":    serviceName,
	}
}

// RunAPIService runs the API service with the given configuration
func (ap *APIPackage) RunAPIService(config interface{}, stopChan chan struct{}) error {
	// We need to perform a dynamic import of the API server module
	// Since we can't directly import it without creating a circular dependency
    
	// Get the path to the API server main.go
	serverMainPath := filepath.Join(ap.apiServiceDir, "cmd", "server", "main.go")
	
	// Check if the file exists
	if _, err := os.Stat(serverMainPath); os.IsNotExist(err) {
		return fmt.Errorf("API server main.go not found at %s", serverMainPath)
	}
	
	// We need to compile and run the API server dynamically
	// For now, we'll use the exec approach but prepare for future direct import
	
	configMap, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid config type")
	}
	
	// Extract config values
	port := configMap["Port"].(int)
	dbURL := configMap["DatabaseURL"].(string)
	useYahoo := configMap["YahooEnabled"].(bool)
	alphaKey := configMap["AlphaKey"].(string)
	yahooHost := configMap["YahooHost"].(string)
	yahooPort := configMap["YahooPort"].(int)
	useHealth := configMap["HealthEnabled"].(bool)
	healthService := configMap["HealthService"].(string)
	serviceName := configMap["ServiceName"].(string)
	
	// Skip the build step since we're going to use "go run" directly
	
	// Build the correct path for the API server
	// We need to use go run directly since the binary isn't working
	
	// Arguments for go run
	args := []string{
		"run", "cmd/main/main.go",
		"--port", strconv.Itoa(port),
		"--db", dbURL,
	}
	
	if useYahoo {
		args = append(args, "--yahoo")
	} else {
		args = append(args, "--yahoo=false")
	}
	
	if alphaKey != "" {
		args = append(args, "--alpha-key", alphaKey)
	}
	
	args = append(args, 
		"--yahoo-host", yahooHost,
		"--yahoo-port", strconv.Itoa(yahooPort))
	
	if useHealth {
		args = append(args, "--health")
		if healthService != "" {
			args = append(args, "--health-service", healthService)
		}
	}
	
	if serviceName != "" {
		args = append(args, "--name", serviceName)
	}
	
	// Prepare command
	cmd := exec.Command("go", args...)
	cmd.Dir = ap.apiServiceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Start the API service
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start API Service: %w", err)
	}
	
	// Set up a goroutine to wait for the stop signal
	go func() {
		// Wait for stop signal
		<-stopChan
		
		// Terminate the process
		if cmd.Process != nil {
			cmd.Process.Signal(os.Interrupt)
			// Wait for a moment to allow graceful shutdown
			time.Sleep(2 * time.Second)
			// Force kill if still running
			cmd.Process.Kill()
		}
	}()
	
	// Wait for the process to complete
	return cmd.Wait()
}

// RunJob runs a scheduler job by executing the scheduler binary with the -run-job flag
func (sp *SchedulerPackage) RunJob(ctx context.Context, jobName, configPath string) error {
	if configPath == "" {
		configPath = filepath.Join(filepath.Dir(sp.schedulerDir), "scheduler-config.json")
	}

	// Check config file
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("scheduler config file not found at %s", configPath)
	}

	// Check scheduler binary
	schedulerBin := filepath.Join(sp.schedulerDir, "scheduler")
	if _, err := os.Stat(schedulerBin); os.IsNotExist(err) {
		return fmt.Errorf("scheduler binary not found at %s", schedulerBin)
	}

	// Run the scheduler with the -run-job flag
	fmt.Printf("Running scheduler job '%s'...\n", jobName)
	cmd := exec.CommandContext(ctx, schedulerBin, 
		"-run-job", jobName,
		"--config", configPath)
	cmd.Dir = sp.schedulerDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command and wait for it to complete
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run scheduler job: %w", err)
	}

	return nil
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