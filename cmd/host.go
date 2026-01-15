package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/weka/amgctl-gds/internal/hardware"
)

var hostCmd = &cobra.Command{
	Use:   "host",
	Short: "Host environment management commands",
	Long:  `Manage and monitor the host environment for AMG.`,
}

var hostPreFlightCmd = &cobra.Command{
	Use:          "pre-flight",
	Short:        "Verify system readiness for AMG setup and execution",
	Long:         `Perform pre-flight checks to ensure the host environment is ready for AMG setup and execution. This includes validating required tools, configurations, and system settings.`,
	SilenceUsage: true, // Don't show help when validation fails
	RunE: func(cmd *cobra.Command, args []string) error {
		full, _ := cmd.Flags().GetBool("full")
		bareSetup, _ := cmd.Flags().GetBool("bare-setup")
		return runHostPreFlight(full, bareSetup)
	},
}

var hostConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
	Long:  `Manage host configuration files and settings for AMG.`,
}

var hostConfigCufileCmd = &cobra.Command{
	Use:   "cufile [output_path]",
	Short: "Configure cufile.json for optimal GPU Direct Storage performance",
	Long: `Copy /etc/cufile.json to the AMG base directory and configure it with optimal settings.

This command will:
- Copy all contents from /etc/cufile.json
- Set execution.max_io_threads to 0
- Set execution.parallel_io to true
- Set execution.max_io_queue_depth to 128 (or keep higher value if already set)
- Set execution.max_request_parallelism to 4 (or keep higher value if already set)
- Set properties.rdma_dev_addr_list to the list of UP InfiniBand IP addresses
- Set properties.allow_compat_mode to true
- Set properties.gds_rdma_write_support to true
- Set fs.weka.rdma_write_support to true

The output_path is optional and defaults to ~/amg_stable/cufile.json.

Examples:
  amgctl host config cufile
  amgctl host config cufile /custom/path/cufile.json
  amgctl host config cufile ~/my_cufile.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var outputPath string
		if len(args) > 0 {
			outputPath = args[0]
		}
		return runHostConfigCufile(outputPath)
	},
}

var hostGdsCmd = &cobra.Command{
	Use:   "gds",
	Short: "GPU Direct Storage (GDS) management commands",
	Long:  `Manage GPU Direct Storage configuration and setup for AMG.`,
}

var hostGdsSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up GPU Direct Storage configuration",
	Long: `Set up GPU Direct Storage by creating and configuring weka-cufile.json for optimal performance.

This command will:
- Copy all contents from /etc/cufile.json
- Set execution.max_io_threads to 0
- Set execution.parallel_io to true
- Set execution.max_io_queue_depth to 128 (or keep higher value if already set)
- Set execution.max_request_parallelism to 4 (or keep higher value if already set)
- Set properties.rdma_dev_addr_list to the list of UP InfiniBand IP addresses
- Set properties.allow_compat_mode to true
- Set properties.gds_rdma_write_support to true
- Set fs.weka.rdma_write_support to true

By default, the weka-cufile.json is created in the current directory, and all output
is logged to both stdout and amgctl-gds-setup.log in the same directory.

Examples:
  amgctl host gds setup
  amgctl host gds setup --config-dir /custom/path
  amgctl host gds setup --pre-flight
  amgctl host gds setup --config-dir /custom/path --pre-flight`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir, _ := cmd.Flags().GetString("config-dir")
		preFlight, _ := cmd.Flags().GetBool("pre-flight")
		bareSetup, _ := cmd.Flags().GetBool("bare-setup")
		return runHostGdsSetup(configDir, preFlight, bareSetup)
	},
}

func init() {
	hostCmd.AddCommand(hostPreFlightCmd)
	hostCmd.AddCommand(hostConfigCmd)
	hostCmd.AddCommand(hostGdsCmd)

	// Add config subcommands
	hostConfigCmd.AddCommand(hostConfigCufileCmd)

	// Add gds subcommands
	hostGdsCmd.AddCommand(hostGdsSetupCmd)

	// Add flags to hostGdsSetupCmd
	hostGdsSetupCmd.Flags().String("config-dir", "", "Directory to output weka-cufile.json and amgctl-gds-setup.log (defaults to current directory)")
	hostGdsSetupCmd.Flags().Bool("pre-flight", false, "Run 'amgctl host pre-flight --full' after creating weka-cufile.json")
	hostGdsSetupCmd.Flags().Bool("bare-setup", false, "Check for uv and git commands during pre-flight (needed for bare-metal setup)")

	// Add flags to hostPreFlightCmd
	hostPreFlightCmd.Flags().Bool("full", false, "Run comprehensive checks including GPU Direct Storage (GDS) validation")
	hostPreFlightCmd.Flags().Bool("bare-setup", false, "Check for uv and git commands (needed for bare-metal setup)")
}

// Configuration constants
const (
	uvEnvName = "amg_stable"
)

func getBasePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "amg_stable")
}

// commandExists checks if a command is available in the system PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// CuFileConfig represents the structure of /etc/cufile.json with all configurable fields
type CuFileConfig struct {
	Execution struct {
		MaxIOThreads          int  `json:"max_io_threads"`
		ParallelIO            bool `json:"parallel_io"`
		MaxIOQueueDepth       int  `json:"max_io_queue_depth"`
		MaxRequestParallelism int  `json:"max_request_parallelism"`
	} `json:"execution"`
	Properties struct {
		RdmaDevAddrList     []string `json:"rdma_dev_addr_list"`
		AllowCompatMode     bool     `json:"allow_compat_mode"`
		GdsRdmaWriteSupport bool     `json:"gds_rdma_write_support"`
	} `json:"properties"`
	FS struct {
		Weka struct {
			RdmaWriteSupport bool `json:"rdma_write_support"`
		} `json:"weka"`
	} `json:"fs"`
}

// runHostSystemChecks performs shared system checks for both setup and pre-flight commands
// bareSetup: if true, checks for uv and git commands (needed for setup)
func runHostSystemChecks(bareSetup bool) error {
	fmt.Println("--- System Checks ---")

	if bareSetup {
		if !commandExists("uv") {
			return fmt.Errorf("uv command not found. Please install uv: https://docs.astral.sh/uv/getting-started/installation/")
		}
		fmt.Println("✅ uv command found")

		if !commandExists("git") {
			return fmt.Errorf("git command not found. Please install Git")
		}
		fmt.Println("✅ git command found")
	}

	if err := checkCuFileConfig(); err != nil {
		// This is a warning, not a fatal error
		fmt.Printf("⚠️  %v\n", err)
	}

	if err := checkNvidiaPeermemModule(); err != nil {
		return fmt.Errorf("nvidia_peermem module check failed: %w", err)
	}

	if err := checkNvidiaFsModule(); err != nil {
		return fmt.Errorf("nvidia_fs module check failed: %w", err)
	}

	fmt.Println("✅ System checks completed")
	return nil
}

// stripJSONComments removes C-style comments from JSON content
func stripJSONComments(jsonData []byte) []byte {
	// Remove single-line comments (//)
	singleLineCommentRe := regexp.MustCompile(`//.*`)
	result := singleLineCommentRe.ReplaceAll(jsonData, []byte(""))

	// Remove multi-line comments (/* */)
	multiLineCommentRe := regexp.MustCompile(`(?s)/\*.*?\*/`)
	result = multiLineCommentRe.ReplaceAll(result, []byte(""))

	return result
}

// checkCuFileConfig validates cufile.json configuration
// First checks CUFILE_ENV_PATH_JSON env var, then basepath directory, then fallback to /etc/cufile.json
func checkCuFileConfig() error {
	var cufilePath string

	// Check if CUFILE_ENV_PATH_JSON is already set (e.g., by gds setup)
	if envPath := os.Getenv("CUFILE_ENV_PATH_JSON"); envPath != "" {
		cufilePath = envPath
		if _, err := os.Stat(cufilePath); os.IsNotExist(err) {
			return fmt.Errorf("cufile.json not found at CUFILE_ENV_PATH_JSON=%s", cufilePath)
		}
	} else {
		// Try basepath directory first
		basePath := getBasePath()
		cufilePath = filepath.Join(basePath, "cufile.json")

		// Check if cufile.json exists in basepath directory
		if _, err := os.Stat(cufilePath); os.IsNotExist(err) {
			// Fallback to /etc/cufile.json if not found in basepath
			cufilePath = "/etc/cufile.json"
			if _, err := os.Stat(cufilePath); os.IsNotExist(err) {
				return fmt.Errorf("cufile.json not found at %s or /etc/cufile.json. Consider configuring CUDA file operations if needed", filepath.Join(basePath, "cufile.json"))
			}
		}
	}

	data, err := os.ReadFile(cufilePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", cufilePath, err)
	}

	cleanData := stripJSONComments(data)

	var config CuFileConfig
	if err := json.Unmarshal(cleanData, &config); err != nil {
		return fmt.Errorf("failed to parse %s: %w", cufilePath, err)
	}

	if config.Execution.MaxIOThreads != 0 {
		return fmt.Errorf("cufile.json warning: execution.max_io_threads is set to %d, but should be 0 for optimal performance", config.Execution.MaxIOThreads)
	}

	fmt.Printf("✅ cufile.json configuration is optimal (execution.max_io_threads = 0) [using %s]\n", cufilePath)
	return nil
}

// checkBpfJitHarden validates the net.core.bpf_jit_harden sysctl setting
func checkBpfJitHarden() error {
	bpfJitHardenPath := "/proc/sys/net/core/bpf_jit_harden"

	data, err := os.ReadFile(bpfJitHardenPath)
	if err != nil {
		// If we can't read it, it might not exist on this kernel version
		return fmt.Errorf("could not read %s: %w. This may be normal on some kernel versions", bpfJitHardenPath, err)
	}

	valueStr := strings.TrimSpace(string(data))
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return fmt.Errorf("could not parse bpf_jit_harden value '%s': %w", valueStr, err)
	}

	if value != 0 {
		return fmt.Errorf("net.core.bpf_jit_harden is set to %d, but should be 0 for optimal performance. To fix: sudo sysctl -w net.core.bpf_jit_harden=0", value)
	}

	fmt.Println("✅ net.core.bpf_jit_harden is optimally configured (0)")
	return nil
}

func checkNvidiaPeermemModule() error {
	moduleName := "nvidia_peermem"

	if err := checkKernelModuleLoaded(moduleName); err == nil {
		fmt.Println("✅ nvidia_peermem module is loaded")
		return nil
	}

	if err := checkKernelModuleExists(moduleName); err != nil {
		return fmt.Errorf("nvidia_peermem module not found. Please install the nvidia_peermem module")
	}

	return fmt.Errorf("nvidia_peermem module found but not loaded. Please load it with: sudo modprobe %s", moduleName)
}

func checkNvidiaFsModule() error {
	moduleName := "nvidia_fs"

	if err := checkKernelModuleLoaded(moduleName); err == nil {
		fmt.Println("✅ nvidia_fs module is loaded")
		return nil
	}

	if err := checkKernelModuleExists(moduleName); err != nil {
		return fmt.Errorf("nvidia_fs module not found. Please install the nvidia_fs module")
	}

	return fmt.Errorf("nvidia_fs module found but not loaded. Please load it with: sudo modprobe %s", moduleName)
}

func checkKernelModuleExists(moduleName string) error {
	cmd := exec.Command("modinfo", moduleName)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("module not found")
	}

	if len(output) == 0 {
		return fmt.Errorf("module exists but modinfo returned no information")
	}

	return nil
}

func checkKernelModuleLoaded(moduleName string) error {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return fmt.Errorf("failed to read /proc/modules: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, moduleName+" ") || strings.HasPrefix(line, moduleName+"\t") {
			return nil
		}
	}

	return fmt.Errorf("module not loaded")
}

// isCondaActive checks if a conda environment is currently active
func isCondaActive() bool {
	condaEnv := os.Getenv("CONDA_DEFAULT_ENV")
	condaPrefix := os.Getenv("CONDA_PREFIX")
	return condaEnv != "" || condaPrefix != ""
}

// checkCondaDeactivated ensures no conda environment is active
func checkCondaDeactivated() error {
	if isCondaActive() {
		return fmt.Errorf("conda environment is currently active. Please deactivate your conda environment before using amgctl host commands:\n  conda deactivate")
	}
	return nil
}

func runHostPreFlight(full bool, bareSetup bool) error {
	if full {
		fmt.Println("🔍 Running comprehensive AMG pre-flight checks...")
	} else {
		fmt.Println("🔍 Running AMG pre-flight checks...")
	}
	fmt.Println()

	if err := checkCondaDeactivated(); err != nil {
		return err
	}

	// Run system checks
	if err := runHostSystemChecks(bareSetup); err != nil {
		return err
	}

	// Run GDS and additional checks if --full flag is enabled
	if full {
		fmt.Println()

		// Check BPF JIT harden setting (warning only)
		if err := checkBpfJitHarden(); err != nil {
			fmt.Printf("⚠️  %v\n", err)
		}

		if err := runGDSChecks(); err != nil {
			return fmt.Errorf("GDS checks failed: %w", err)
		}
	}

	fmt.Println()
	fmt.Println("🎉 Pre-flight checks completed successfully!")

	return nil
}

// runGDSChecks performs GPU Direct Storage checks using gdscheck
func runGDSChecks() error {
	fmt.Println("--- GPU Direct Storage (GDS) Checks ---")

	gdsCheckPath := "/usr/local/cuda/gds/tools/gdscheck"

	// Check if gdscheck tool exists
	if _, err := os.Stat(gdsCheckPath); os.IsNotExist(err) {
		return fmt.Errorf("gdscheck tool not found at %s. GPU Direct Storage may not be installed", gdsCheckPath)
	}

	fmt.Printf("✅ Found gdscheck tool at %s\n", gdsCheckPath)
	fmt.Println("Running GDS platform checks...")

	// Run gdscheck -p
	cmd := exec.Command(gdsCheckPath, "-p")

	// Check for cufile.json and add CUFILE_ENV_PATH_JSON if it exists
	// First check if CUFILE_ENV_PATH_JSON is already set (e.g., by gds setup)
	var cufilePath string
	if envPath := os.Getenv("CUFILE_ENV_PATH_JSON"); envPath != "" {
		cufilePath = envPath
		fmt.Printf("✅ Using CUFILE_ENV_PATH_JSON=%s for gdscheck\n", cufilePath)
	} else {
		basePath := getBasePath()
		cufilePath = filepath.Join(basePath, "cufile.json")
		if _, err := os.Stat(cufilePath); err == nil {
			fmt.Printf("✅ Found cufile.json, setting CUFILE_ENV_PATH_JSON=%s for gdscheck\n", cufilePath)
		} else {
			cufilePath = "" // No cufile found
		}
	}

	// Set environment for gdscheck if we have a cufile path
	if cufilePath != "" {
		env := os.Environ()
		// Remove any existing CUFILE_ENV_PATH_JSON from environment
		var filteredEnv []string
		for _, e := range env {
			if !strings.HasPrefix(e, "CUFILE_ENV_PATH_JSON=") {
				filteredEnv = append(filteredEnv, e)
			}
		}
		filteredEnv = append(filteredEnv, fmt.Sprintf("CUFILE_ENV_PATH_JSON=%s", cufilePath))
		cmd.Env = filteredEnv
	}

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run gdscheck: %w", err)
	}

	outputStr := string(output)

	// Parse and validate the output
	if err := validateGDSOutput(outputStr); err != nil {
		return err
	}

	// Check for gdsio tool (warning only)
	gdsioPath := "/usr/local/cuda/gds/tools/gdsio"
	if _, err := os.Stat(gdsioPath); os.IsNotExist(err) {
		fmt.Printf("⚠️  gdsio tool not found at %s. Consider installing GDS IO utilities for performance testing\n", gdsioPath)
	} else {
		fmt.Printf("✅ Found gdsio tool at %s\n", gdsioPath)
	}

	fmt.Println("✅ GDS checks completed successfully")
	return nil
}

// validateGDSOutput parses gdscheck output and validates required components
func validateGDSOutput(output string) error {
	lines := strings.Split(output, "\n")

	// Track requirements
	requirements := map[string]bool{
		"wekafs_supported":            false,
		"userspace_rdma_supported":    false,
		"mellanox_peerdirect_enabled": false,
		"rdma_library_loaded":         false,
		"rdma_devices_configured":     false,
		"iommu_disabled":              false,
	}

	// Parse the output line by line
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check WekaFS support
		if strings.Contains(line, "WekaFS") && strings.Contains(line, ": Supported") {
			requirements["wekafs_supported"] = true
		}

		// Check Userspace RDMA support
		if strings.Contains(line, "Userspace RDMA") && strings.Contains(line, ": Supported") {
			requirements["userspace_rdma_supported"] = true
		}

		// Check Mellanox PeerDirect
		if strings.Contains(line, "--Mellanox PeerDirect") && strings.Contains(line, ": Enabled") {
			requirements["mellanox_peerdirect_enabled"] = true
		}

		// Check rdma library
		if strings.Contains(line, "--rdma library") && strings.Contains(line, ": Loaded") {
			requirements["rdma_library_loaded"] = true
		}

		// Check rdma devices
		if strings.Contains(line, "--rdma devices") && strings.Contains(line, ": Configured") {
			requirements["rdma_devices_configured"] = true
		}

		// Check IOMMU status
		if strings.Contains(line, "IOMMU: disabled") {
			requirements["iommu_disabled"] = true
		}
	}

	// Validate all requirements
	var errors []string

	if !requirements["wekafs_supported"] {
		errors = append(errors, "WekaFS is not supported")
	} else {
		fmt.Println("✅ WekaFS: Supported")
	}

	if !requirements["userspace_rdma_supported"] {
		errors = append(errors, "Userspace RDMA is not supported")
	} else {
		fmt.Println("✅ Userspace RDMA: Supported")
	}

	if !requirements["mellanox_peerdirect_enabled"] {
		errors = append(errors, "Mellanox PeerDirect is not enabled")
	} else {
		fmt.Println("✅ Mellanox PeerDirect: Enabled")
	}

	if !requirements["rdma_library_loaded"] {
		errors = append(errors, "RDMA library is not loaded")
	} else {
		fmt.Println("✅ RDMA library: Loaded")
	}

	if !requirements["rdma_devices_configured"] {
		errors = append(errors, "RDMA devices are not configured")
	} else {
		fmt.Println("✅ RDMA devices: Configured")
	}

	// IOMMU check - warn if not disabled but don't fail
	if !requirements["iommu_disabled"] {
		fmt.Println("⚠️  IOMMU: Not disabled - performance may not be optimal")
	} else {
		fmt.Println("✅ IOMMU: Disabled")
	}

	// Return combined errors if any
	if len(errors) > 0 {
		return fmt.Errorf("GDS validation failed:\n  • %s", strings.Join(errors, "\n  • "))
	}

	return nil
}

// runHostConfigCufile copies and configures cufile.json for optimal performance
// If outputPath is empty, defaults to ~/amg_stable/cufile.json
func runHostConfigCufile(outputPath string) error {
	fmt.Println("🔧 Configuring cufile.json for optimal GPU Direct Storage performance...")
	fmt.Println()

	// Check if source file exists
	sourcePath := "/etc/cufile.json"
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source file %s does not exist", sourcePath)
	}

	// Determine destination path
	var destPath string
	if outputPath != "" {
		// Expand tilde if present
		if strings.HasPrefix(outputPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			destPath = filepath.Join(home, outputPath[2:])
		} else {
			destPath = outputPath
		}
	} else {
		// Default path
		basePath := getBasePath()
		if err := os.MkdirAll(basePath, 0755); err != nil {
			return fmt.Errorf("failed to create base directory %s: %w", basePath, err)
		}
		destPath = filepath.Join(basePath, "cufile.json")
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	// Read and parse source file
	fmt.Printf("📖 Reading source file: %s\n", sourcePath)
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %w", sourcePath, err)
	}

	// Strip comments and parse JSON as a map to preserve all fields
	cleanData := stripJSONComments(sourceData)
	var config map[string]interface{}
	if err := json.Unmarshal(cleanData, &config); err != nil {
		return fmt.Errorf("failed to parse source file %s: %w", sourcePath, err)
	}

	fmt.Println("✅ Successfully parsed source cufile.json")

	// Get InfiniBand UP IP addresses
	fmt.Println("🔍 Detecting InfiniBand network interfaces...")
	ibInterfaces, err := hardware.GetInfiniBandNetworkInterfaces()
	if err != nil {
		return fmt.Errorf("failed to get InfiniBand interfaces: %w", err)
	}

	var upIPs []string
	for _, iface := range ibInterfaces {
		if iface.Status == "up" && iface.IPAddress != "no IP assigned" {
			// Extract just the IP part (remove CIDR notation if present)
			ip := iface.IPAddress
			if strings.Contains(ip, "/") {
				parts := strings.Split(ip, "/")
				ip = parts[0]
			}
			upIPs = append(upIPs, ip)
		}
	}

	if len(upIPs) == 0 {
		fmt.Println("⚠️  No UP InfiniBand interfaces with IP addresses found")
	} else {
		fmt.Printf("✅ Found %d UP InfiniBand interfaces: %v\n", len(upIPs), upIPs)
	}

	// Configure the values according to requirements
	fmt.Println("⚙️  Configuring optimal settings...")

	// Helper function to get nested map, creating if necessary
	ensureMap := func(parent map[string]interface{}, key string) map[string]interface{} {
		if val, ok := parent[key]; ok {
			if m, ok := val.(map[string]interface{}); ok {
				return m
			}
		}
		newMap := make(map[string]interface{})
		parent[key] = newMap
		return newMap
	}

	// Helper function to get int value from map
	getInt := func(m map[string]interface{}, key string) int {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case int:
				return v
			case float64:
				return int(v)
			}
		}
		return 0
	}

	// Helper function to get bool value from map
	getBool := func(m map[string]interface{}, key string) bool {
		if val, ok := m[key]; ok {
			if b, ok := val.(bool); ok {
				return b
			}
		}
		return false
	}

	// Ensure nested structures exist
	execution := ensureMap(config, "execution")
	properties := ensureMap(config, "properties")
	fs := ensureMap(config, "fs")
	weka := ensureMap(fs, "weka")

	// execution.max_io_threads to be 0
	currentMaxIOThreads := getInt(execution, "max_io_threads")
	if currentMaxIOThreads != 0 {
		fmt.Printf("  Setting execution.max_io_threads: %d → 0\n", currentMaxIOThreads)
		execution["max_io_threads"] = 0
	} else {
		fmt.Println("  execution.max_io_threads: already set to 0 ✅")
	}

	// execution.parallel_io to be true
	currentParallelIO := getBool(execution, "parallel_io")
	if !currentParallelIO {
		fmt.Printf("  Setting execution.parallel_io: %t → true\n", currentParallelIO)
		execution["parallel_io"] = true
	} else {
		fmt.Println("  execution.parallel_io: already set to true ✅")
	}

	// execution.max_io_queue_depth to be 128 (or keep higher)
	currentMaxIOQueueDepth := getInt(execution, "max_io_queue_depth")
	if currentMaxIOQueueDepth < 128 {
		fmt.Printf("  Setting execution.max_io_queue_depth: %d → 128\n", currentMaxIOQueueDepth)
		execution["max_io_queue_depth"] = 128
	} else {
		fmt.Printf("  execution.max_io_queue_depth: keeping higher value %d ✅\n", currentMaxIOQueueDepth)
	}

	// execution.max_request_parallelism to be 4 (or keep higher)
	currentMaxRequestParallelism := getInt(execution, "max_request_parallelism")
	if currentMaxRequestParallelism < 4 {
		fmt.Printf("  Setting execution.max_request_parallelism: %d → 4\n", currentMaxRequestParallelism)
		execution["max_request_parallelism"] = 4
	} else {
		fmt.Printf("  execution.max_request_parallelism: keeping higher value %d ✅\n", currentMaxRequestParallelism)
	}

	// properties.rdma_dev_addr_list to be the list of UP IB IPs
	var currentRdmaDevAddrList []string
	if val, ok := properties["rdma_dev_addr_list"]; ok {
		if arr, ok := val.([]interface{}); ok {
			for _, item := range arr {
				if str, ok := item.(string); ok {
					currentRdmaDevAddrList = append(currentRdmaDevAddrList, str)
				}
			}
		}
	}
	fmt.Printf("  Setting properties.rdma_dev_addr_list: %v → %v\n", currentRdmaDevAddrList, upIPs)
	properties["rdma_dev_addr_list"] = upIPs

	// properties.allow_compat_mode to be true
	currentAllowCompatMode := getBool(properties, "allow_compat_mode")
	if !currentAllowCompatMode {
		fmt.Printf("  Setting properties.allow_compat_mode: %t → true\n", currentAllowCompatMode)
		properties["allow_compat_mode"] = true
	} else {
		fmt.Println("  properties.allow_compat_mode: already set to true ✅")
	}

	// properties.gds_rdma_write_support to be true
	currentGdsRdmaWriteSupport := getBool(properties, "gds_rdma_write_support")
	if !currentGdsRdmaWriteSupport {
		fmt.Printf("  Setting properties.gds_rdma_write_support: %t → true\n", currentGdsRdmaWriteSupport)
		properties["gds_rdma_write_support"] = true
	} else {
		fmt.Println("  properties.gds_rdma_write_support: already set to true ✅")
	}

	// fs.weka.rdma_write_support to be true
	currentRdmaWriteSupport := getBool(weka, "rdma_write_support")
	if !currentRdmaWriteSupport {
		fmt.Printf("  Setting fs.weka.rdma_write_support: %t → true\n", currentRdmaWriteSupport)
		weka["rdma_write_support"] = true
	} else {
		fmt.Println("  fs.weka.rdma_write_support: already set to true ✅")
	}

	// Write the configured JSON to destination
	fmt.Printf("💾 Writing configured cufile.json to: %s\n", destPath)

	// Marshal with proper indentation
	outputData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := os.WriteFile(destPath, outputData, 0644); err != nil {
		return fmt.Errorf("failed to write destination file %s: %w", destPath, err)
	}

	fmt.Printf("✅ Successfully created optimized cufile.json at %s\n", destPath)
	fmt.Println()
	fmt.Println("🎉 cufile.json configuration completed!")
	fmt.Println("💡 vLLM will automatically use this configuration when CUFILE_ENV_PATH_JSON is set")

	return nil
}

// logWriter wraps multiple io.Writers to write to both stdout and a log file
type logWriter struct {
	stdout  *os.File
	logFile *os.File
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	// Write to stdout
	n1, err1 := lw.stdout.Write(p)
	// Write to log file
	n2, err2 := lw.logFile.Write(p)

	// Return the minimum bytes written and any error
	if err1 != nil {
		return n1, err1
	}
	if err2 != nil {
		return n2, err2
	}
	if n1 < n2 {
		return n1, nil
	}
	return n2, nil
}

// runHostGdsSetup sets up GPU Direct Storage configuration
func runHostGdsSetup(configDir string, preFlight bool, bareSetup bool) error {
	// Determine output directory
	var outputDir string
	if configDir != "" {
		outputDir = configDir
	} else {
		// Use current working directory
		var err error
		outputDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	// Open log file
	logFilePath := filepath.Join(outputDir, "amgctl-gds-setup.log")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("failed to create log file %s: %w", logFilePath, err)
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close log file: %v\n", err)
		}
	}()

	// Create logWriter to write to both stdout and log file
	lw := &logWriter{
		stdout:  os.Stdout,
		logFile: logFile,
	}

	// Redirect fmt output to our logWriter
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	os.Stdout = w

	// Start a goroutine to copy from pipe to our logWriter
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				if _, writeErr := lw.Write(buf[:n]); writeErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to write to log: %v\n", writeErr)
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Ensure we restore stdout and close pipe
	defer func() {
		if err := w.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close pipe writer: %v\n", err)
		}
		<-done
		os.Stdout = oldStdout
	}()

	// Determine destination path for weka-cufile.json
	cufilePath := filepath.Join(outputDir, "weka-cufile.json")

	fmt.Println("🔧 Setting up GPU Direct Storage configuration...")
	fmt.Println()
	fmt.Printf("📁 Output directory: %s\n", outputDir)
	fmt.Printf("📝 Log file: %s\n", logFilePath)
	fmt.Printf("📄 Cufile path: %s\n", cufilePath)
	fmt.Println()

	// Check if source file exists
	sourcePath := "/etc/cufile.json"
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source file %s does not exist", sourcePath)
	}

	// Read and parse source file
	fmt.Printf("📖 Reading source file: %s\n", sourcePath)
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %w", sourcePath, err)
	}

	// Strip comments and parse JSON as a map to preserve all fields
	cleanData := stripJSONComments(sourceData)
	var config map[string]interface{}
	if err := json.Unmarshal(cleanData, &config); err != nil {
		return fmt.Errorf("failed to parse source file %s: %w", sourcePath, err)
	}

	fmt.Println("✅ Successfully parsed source cufile.json")

	// Get InfiniBand UP IP addresses
	fmt.Println("🔍 Detecting InfiniBand network interfaces...")
	ibInterfaces, err := hardware.GetInfiniBandNetworkInterfaces()
	if err != nil {
		return fmt.Errorf("failed to get InfiniBand interfaces: %w", err)
	}

	var upIPs []string
	for _, iface := range ibInterfaces {
		if iface.Status == "up" && iface.IPAddress != "no IP assigned" {
			// Extract just the IP part (remove CIDR notation if present)
			ip := iface.IPAddress
			if strings.Contains(ip, "/") {
				parts := strings.Split(ip, "/")
				ip = parts[0]
			}
			upIPs = append(upIPs, ip)
		}
	}

	if len(upIPs) == 0 {
		fmt.Println("⚠️  No UP InfiniBand interfaces with IP addresses found")
	} else {
		fmt.Printf("✅ Found %d UP InfiniBand interfaces: %v\n", len(upIPs), upIPs)
	}

	// Configure the values according to requirements
	fmt.Println("⚙️  Configuring optimal settings...")

	// Helper function to get nested map, creating if necessary
	ensureMap := func(parent map[string]interface{}, key string) map[string]interface{} {
		if val, ok := parent[key]; ok {
			if m, ok := val.(map[string]interface{}); ok {
				return m
			}
		}
		newMap := make(map[string]interface{})
		parent[key] = newMap
		return newMap
	}

	// Helper function to get int value from map
	getInt := func(m map[string]interface{}, key string) int {
		if val, ok := m[key]; ok {
			switch v := val.(type) {
			case int:
				return v
			case float64:
				return int(v)
			}
		}
		return 0
	}

	// Helper function to get bool value from map
	getBool := func(m map[string]interface{}, key string) bool {
		if val, ok := m[key]; ok {
			if b, ok := val.(bool); ok {
				return b
			}
		}
		return false
	}

	// Ensure nested structures exist
	execution := ensureMap(config, "execution")
	properties := ensureMap(config, "properties")
	fs := ensureMap(config, "fs")
	weka := ensureMap(fs, "weka")

	// execution.max_io_threads to be 0
	currentMaxIOThreads := getInt(execution, "max_io_threads")
	if currentMaxIOThreads != 0 {
		fmt.Printf("  Setting execution.max_io_threads: %d → 0\n", currentMaxIOThreads)
		execution["max_io_threads"] = 0
	} else {
		fmt.Println("  execution.max_io_threads: already set to 0 ✅")
	}

	// execution.parallel_io to be true
	currentParallelIO := getBool(execution, "parallel_io")
	if !currentParallelIO {
		fmt.Printf("  Setting execution.parallel_io: %t → true\n", currentParallelIO)
		execution["parallel_io"] = true
	} else {
		fmt.Println("  execution.parallel_io: already set to true ✅")
	}

	// execution.max_io_queue_depth to be 128 (or keep higher)
	currentMaxIOQueueDepth := getInt(execution, "max_io_queue_depth")
	if currentMaxIOQueueDepth < 128 {
		fmt.Printf("  Setting execution.max_io_queue_depth: %d → 128\n", currentMaxIOQueueDepth)
		execution["max_io_queue_depth"] = 128
	} else {
		fmt.Printf("  execution.max_io_queue_depth: keeping higher value %d ✅\n", currentMaxIOQueueDepth)
	}

	// execution.max_request_parallelism to be 4 (or keep higher)
	currentMaxRequestParallelism := getInt(execution, "max_request_parallelism")
	if currentMaxRequestParallelism < 4 {
		fmt.Printf("  Setting execution.max_request_parallelism: %d → 4\n", currentMaxRequestParallelism)
		execution["max_request_parallelism"] = 4
	} else {
		fmt.Printf("  execution.max_request_parallelism: keeping higher value %d ✅\n", currentMaxRequestParallelism)
	}

	// properties.rdma_dev_addr_list to be the list of UP IB IPs
	var currentRdmaDevAddrList []string
	if val, ok := properties["rdma_dev_addr_list"]; ok {
		if arr, ok := val.([]interface{}); ok {
			for _, item := range arr {
				if str, ok := item.(string); ok {
					currentRdmaDevAddrList = append(currentRdmaDevAddrList, str)
				}
			}
		}
	}
	fmt.Printf("  Setting properties.rdma_dev_addr_list: %v → %v\n", currentRdmaDevAddrList, upIPs)
	properties["rdma_dev_addr_list"] = upIPs

	// properties.allow_compat_mode to be true
	currentAllowCompatMode := getBool(properties, "allow_compat_mode")
	if !currentAllowCompatMode {
		fmt.Printf("  Setting properties.allow_compat_mode: %t → true\n", currentAllowCompatMode)
		properties["allow_compat_mode"] = true
	} else {
		fmt.Println("  properties.allow_compat_mode: already set to true ✅")
	}

	// properties.gds_rdma_write_support to be true
	currentGdsRdmaWriteSupport := getBool(properties, "gds_rdma_write_support")
	if !currentGdsRdmaWriteSupport {
		fmt.Printf("  Setting properties.gds_rdma_write_support: %t → true\n", currentGdsRdmaWriteSupport)
		properties["gds_rdma_write_support"] = true
	} else {
		fmt.Println("  properties.gds_rdma_write_support: already set to true ✅")
	}

	// fs.weka.rdma_write_support to be true
	currentRdmaWriteSupport := getBool(weka, "rdma_write_support")
	if !currentRdmaWriteSupport {
		fmt.Printf("  Setting fs.weka.rdma_write_support: %t → true\n", currentRdmaWriteSupport)
		weka["rdma_write_support"] = true
	} else {
		fmt.Println("  fs.weka.rdma_write_support: already set to true ✅")
	}

	// Write the configured JSON to destination
	fmt.Printf("💾 Writing configured weka-cufile.json to: %s\n", cufilePath)

	// Marshal with proper indentation
	outputData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := os.WriteFile(cufilePath, outputData, 0644); err != nil {
		return fmt.Errorf("failed to write destination file %s: %w", cufilePath, err)
	}

	fmt.Printf("✅ Successfully created optimized weka-cufile.json at %s\n", cufilePath)
	fmt.Println()
	fmt.Println("🎉 GDS setup completed!")
	fmt.Printf("📝 All output has been logged to: %s\n", logFilePath)
	fmt.Println()

	// Run pre-flight check if requested
	if preFlight {
		fmt.Println("🚀 Running pre-flight checks...")
		fmt.Println()

		// Set CUFILE_ENV_PATH_JSON environment variable for pre-flight checks
		oldCufileEnv := os.Getenv("CUFILE_ENV_PATH_JSON")
		if err := os.Setenv("CUFILE_ENV_PATH_JSON", cufilePath); err != nil {
			return fmt.Errorf("failed to set CUFILE_ENV_PATH_JSON: %w", err)
		}
		defer func() {
			if oldCufileEnv != "" {
				if err := os.Setenv("CUFILE_ENV_PATH_JSON", oldCufileEnv); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to restore CUFILE_ENV_PATH_JSON: %v\n", err)
				}
			} else {
				if err := os.Unsetenv("CUFILE_ENV_PATH_JSON"); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to unset CUFILE_ENV_PATH_JSON: %v\n", err)
				}
			}
		}()

		if err := runHostPreFlight(true, bareSetup); err != nil {
			return fmt.Errorf("pre-flight checks failed: %w", err)
		}
	}

	return nil
}
