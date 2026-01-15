# amgctl-gds - Minimal GDS Configuration Tool

This is a lightweight, focused version of `amgctl` that contains **only** GPU Direct Storage (GDS) setup and configuration functionality.

## Overview

`amgctl-gds` provides the essential commands needed to configure and validate GPU Direct Storage environments:

- Configure `cufile.json` with optimal GDS settings
- Detect and configure InfiniBand network interfaces  
- Validate GDS prerequisites (kernel modules, RDMA, etc.)
- Run comprehensive pre-flight checks

## Key Differences from Full amgctl

**Included:**
- ✅ `host config cufile` - cufile.json configuration
- ✅ `host gds setup` - GDS setup with logging
- ✅ `host pre-flight` - System validation

**Removed:**
- ❌ vLLM launching
- ❌ Environment setup/installation
- ❌ Docker/Kubernetes integration
- ❌ Status/update commands
- ❌ GPU detection

**Benefits:**
- ~70% smaller codebase (~1,400 lines vs ~4,600 lines)
- ~60% smaller binary (~8-12 MB vs ~20-30 MB)
- Faster build times
- Single-purpose, easier to maintain

## Installation

### Build from Source

```bash
make build
```

The binary will be created at `build/amgctl`.

### Install System-wide

```bash
make install
```

This installs the binary to `/usr/local/bin/amgctl`.

## Commands

### 1. Configure cufile.json

Configure `cufile.json` with optimal GDS settings and InfiniBand network interfaces:

```bash
# Use default path (~/amg_stable/cufile.json)
amgctl host config cufile

# Specify custom output path
amgctl host config cufile /custom/path/cufile.json
amgctl host config cufile ~/my_cufile.json
```

**What it does:**
- Reads `/etc/cufile.json` as the source
- Detects active InfiniBand interfaces and their IP addresses
- Applies optimal GDS performance settings:
  - `execution.max_io_threads` → 0
  - `execution.parallel_io` → true
  - `execution.max_io_queue_depth` → 128 (or keeps higher value)
  - `execution.max_request_parallelism` → 4 (or keeps higher value)
  - `properties.rdma_dev_addr_list` → detected IB IPs
  - `properties.allow_compat_mode` → true
  - `properties.gds_rdma_write_support` → true
  - `fs.weka.rdma_write_support` → true
- Outputs configured file to specified path

### 2. GDS Setup with Logging

Comprehensive GDS setup that creates `weka-cufile.json` and logs all output:

```bash
# Setup in current directory
amgctl host gds setup

# Setup in custom directory
amgctl host gds setup --config-dir /custom/path

# Setup with pre-flight validation
amgctl host gds setup --pre-flight

# Full example
amgctl host gds setup --config-dir /opt/gds --pre-flight
```

**What it does:**
- Creates `weka-cufile.json` in the specified directory
- Logs all output to `amgctl-gds-setup.log` in the same directory
- Applies the same optimal GDS settings as `config cufile`
- Optionally runs pre-flight checks after setup (with `--pre-flight`)

### 3. Pre-flight Validation

Validate that the system is ready for GDS operations:

```bash
# Basic checks
amgctl host pre-flight

# Comprehensive checks including gdscheck
amgctl host pre-flight --full

# Check for bare-metal setup tools (uv, git)
amgctl host pre-flight --bare-setup
```

**What it checks:**

**Basic checks:**
- `nvidia_peermem` kernel module loaded
- `nvidia_fs` kernel module loaded  
- `cufile.json` exists and has optimal settings

**Full checks (with `--full`):**
- All basic checks
- Runs `/usr/local/cuda/gds/tools/gdscheck -p`
- Validates:
  - WekaFS support
  - Userspace RDMA support
  - Mellanox PeerDirect enabled
  - RDMA library loaded
  - RDMA devices configured
  - IOMMU status
- Checks BPF JIT harden setting
- Verifies `gdsio` tool availability

## Requirements

### System Dependencies

- `/etc/cufile.json` (source configuration)
- Active InfiniBand interfaces (for automatic IP detection)
- NVIDIA GDS installed (for `gdscheck` tool)
- Required kernel modules:
  - `nvidia_fs`
  - `nvidia_peermem`
  - RDMA modules (`mlx5_ib`, `rdma_cm`, `ib_core`)

### Go Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `github.com/prometheus/procfs` - sysfs access for InfiniBand detection

## Examples

### Quick GDS Setup

```bash
# 1. Configure cufile.json
amgctl host config cufile

# 2. Validate the setup
amgctl host pre-flight --full
```

### Production Setup with Logging

```bash
# Create GDS configuration with full logging and validation
amgctl host gds setup --config-dir /opt/gds-config --pre-flight

# Check the log
cat /opt/gds-config/amgctl-gds-setup.log

# Verify the configuration
cat /opt/gds-config/weka-cufile.json | jq '.properties.rdma_dev_addr_list'
```

### Custom Path Configuration

```bash
# Configure cufile.json to a specific location
amgctl host config cufile /mnt/weka/config/cufile.json

# Verify InfiniBand IPs were detected
cat /mnt/weka/config/cufile.json | jq '.properties.rdma_dev_addr_list'
```

## Troubleshooting

### No InfiniBand Interfaces Found

If you see "No UP InfiniBand interfaces with IP addresses found":

```bash
# Check InfiniBand interface status
ibstat

# Verify interfaces are UP
ip link show

# Ensure IP addresses are assigned
ip addr show
```

### Kernel Modules Not Loaded

If pre-flight checks fail for kernel modules:

```bash
# Load required modules
sudo modprobe nvidia_fs
sudo modprobe nvidia_peermem

# Verify they're loaded
lsmod | grep nvidia
```

### gdscheck Not Found

If GDS checks fail:

```bash
# Check if GDS is installed
ls -la /usr/local/cuda/gds/tools/

# Install GDS (Ubuntu/Debian)
sudo apt-get install nvidia-gds

# Or install from CUDA toolkit
```

## Default Paths

- **Default cufile.json output:** `~/amg_stable/cufile.json`
- **GDS setup output:** `./weka-cufile.json` (current directory)
- **GDS setup log:** `./amgctl-gds-setup.log` (current directory)

## Drop-in Replacement

This tool maintains **identical command structure** to the full `amgctl`. If you have existing scripts or workflows that use the GDS commands, they will continue to work without modification:

```bash
# These commands work exactly the same:
amgctl host config cufile                    # ✅ Works
amgctl host gds setup --pre-flight           # ✅ Works  
amgctl host pre-flight --full                # ✅ Works

# These commands are not available (removed):
amgctl host launch                           # ❌ Removed
amgctl host setup                            # ❌ Removed
amgctl docker                                # ❌ Removed
amgctl k8s                                   # ❌ Removed
```

## Version

Current version: `0.1.21-gds` (GDS-only variant)

Based on `amgctl` version `0.1.21`

## License

Same license as the parent `amg-utils` project.
