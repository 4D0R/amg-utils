#!/usr/bin/env bash
# Quick sanity check for H100 + IB + WEKA tuning on h100a
# - Kernel cmdline flags
# - tuned-adm profile
# - C-states / cpuidle
# - CPU governors
# - irqbalance
# - mlx5_0 NUMA + IB link rate
# - RDMA / NVIDIA modules
# - WEKA mount line

set -u

GREEN="\e[32m"
RED="\e[31m"
YELLOW="\e[33m"
NC="\e[0m"

fail_count=0

check() {
  local ok=$1
  local msg="$2"
  if [ "$ok" -eq 0 ]; then
    echo -e "${GREEN}[PASS]${NC} $msg"
  else
    echo -e "${RED}[FAIL]${NC} $msg"
    fail_count=$((fail_count+1))
  fi
}

echo "===== H100A TUNING VERIFICATION ====="
echo "Host: $(hostname)"
echo "Date: $(date)"
echo "Kernel: $(uname -r)"
echo

########################
# 1) Kernel cmdline
########################
CMDLINE=$(cat /proc/cmdline)
echo ">> /proc/cmdline:"
echo "   $CMDLINE"
echo

for flag in \
  "transparent_hugepage=madvise" \
  "iommu=pt" \
  "pci=realloc=off" \
  "intel_idle.max_cstate=0" \
  "pcie_aspm=off"
do
  if echo "$CMDLINE" | grep -qw "$flag"; then
    check 0 "Kernel flag present: $flag"
  else
    check 1 "Kernel flag MISSING: $flag"
  fi
done
echo

########################
# 2) tuned-adm profile
########################
if command -v tuned-adm >/dev/null 2>&1; then
  ACTIVE_TUNED=$(tuned-adm active 2>/dev/null | awk -F': ' '/Current active profile/ {print $2}')
  echo ">> tuned-adm active: ${ACTIVE_TUNED:-unknown}"
  if [ "${ACTIVE_TUNED:-}" = "throughput-performance" ]; then
    check 0 "tuned-adm profile is throughput-performance"
  else
    check 1 "tuned-adm profile is NOT throughput-performance (got: ${ACTIVE_TUNED:-none})"
  fi
else
  check 1 "tuned-adm not found"
fi
echo

########################
# 3) C-states / cpuidle
########################
if [ -f /sys/module/intel_idle/parameters/max_cstate ]; then
  MCSTATE=$(cat /sys/module/intel_idle/parameters/max_cstate)
  echo ">> intel_idle max_cstate: $MCSTATE"
  if [ "$MCSTATE" = "0" ]; then
    check 0 "intel_idle.max_cstate is 0"
  else
    check 1 "intel_idle.max_cstate is NOT 0 (got $MCSTATE)"
  fi
else
  check 1 "intel_idle max_cstate parameter not found"
fi

# Verify all deeper C-states disabled
MISSING_CPUIDLE=0
for s in /sys/devices/system/cpu/cpu*/cpuidle/state[1-9]/disable; do
  [ -f "$s" ] || continue
  val=$(cat "$s")
  if [ "$val" != "1" ]; then
    echo "  cpuidle state not disabled: $s (value=$val)"
    MISSING_CPUIDLE=1
  fi
done
if [ "$MISSING_CPUIDLE" -eq 0 ]; then
  check 0 "All cpuidle state[1-9] disable=1"
else
  check 1 "Some cpuidle state[1-9] are NOT disabled"
fi
echo

########################
# 4) CPU frequency governor
########################
GOV_FAIL=0
FIRST_BAD=""
for g in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do
  [ -f "$g" ] || continue
  gov=$(cat "$g")
  if [ "$gov" != "performance" ]; then
    GOV_FAIL=1
    FIRST_BAD="$g=$gov"
    break
  fi
done

if [ "$GOV_FAIL" -eq 0 ]; then
  check 0 "All CPU scaling_governor = performance"
else
  check 1 "Some CPUs not on performance governor (e.g. $FIRST_BAD)"
fi
echo

########################
# 5) irqbalance
########################
if command -v systemctl >/dev/null 2>&1; then
  IRQ_STATE=$(systemctl is-active irqbalance 2>/dev/null || true)
  IRQ_ENABLED=$(systemctl is-enabled irqbalance 2>/dev/null || true)
  echo ">> irqbalance state: $IRQ_STATE (enabled: $IRQ_ENABLED)"
  if [ "$IRQ_STATE" = "active" ]; then
    check 1 "irqbalance is ACTIVE (should be stopped for tuned IRQ pinning)"
  else
    check 0 "irqbalance is not active"
  fi
else
  check 1 "systemctl not available to check irqbalance"
fi
echo

########################
# 6) NUMA / mlx5_0
########################
if [ -d /sys/class/infiniband/mlx5_0/device ]; then
  NODE=$(cat /sys/class/infiniband/mlx5_0/device/numa_node)
  echo ">> mlx5_0 NUMA node: $NODE"
  lscpu | grep -E "^NUMA node[01] CPU" || true

  if [ "$NODE" -eq 0 ]; then
    check 0 "mlx5_0 is on NUMA node 0 (expected)"
  else
    check 1 "mlx5_0 NUMA node is $NODE (expected 0)"
  fi
else
  check 1 "mlx5_0 device sysfs path not found"
fi
echo

########################
# 7) IB link state / rate
########################
if command -v ibstat >/dev/null 2>&1; then
  echo ">> ibstat mlx5_0 (State/Rate):"
  ibstat mlx5_0 2>/dev/null | egrep 'State|Rate' || true

  STATE=$(ibstat mlx5_0 2>/dev/null | awk -F': ' '/State/ {print $2}')
  RATE=$(ibstat mlx5_0 2>/dev/null  | awk -F': ' '/Rate/ {print $2}')
  [ -n "$STATE" ] || STATE="unknown"
  [ -n "$RATE" ] || RATE="unknown"

  if [ "$STATE" = "Active" ]; then
    check 0 "mlx5_0 link state is Active"
  else
    check 1 "mlx5_0 link state is NOT Active (got $STATE)"
  fi

  case "$RATE" in
    400*|*400*Gb*|*400*Gbps*)
      check 0 "mlx5_0 link rate looks like 400 Gb/s ($RATE)"
      ;;
    *)
      check 1 "mlx5_0 link rate not 400Gb/s (got $RATE)"
      ;;
  esac
else
  check 1 "ibstat not found"
fi
echo

########################
# 8) RDMA / NVIDIA modules
########################
echo ">> lsmod | grep -E 'nvidia_fs|nvidia_peermem|nvme|rdma_cm|mlx5_ib'"
lsmod | grep -E 'nvidia_fs|nvidia_peermem|nvme|rdma_cm|mlx5_ib' || true
echo

for mod in nvidia nvidia_fs nvidia_peermem rdma_cm mlx5_ib ib_core nvme nvme_fabrics; do
  if lsmod | awk '{print $1}' | grep -qw "$mod"; then
    check 0 "Kernel module loaded: $mod"
  else
    check 1 "Kernel module NOT loaded: $mod"
  fi
done
echo

########################
# 9) GDS capabilities
########################
if command -v gdscheck >/dev/null 2>&1; then
  echo ">> gdscheck -p (grep WekaFS / Userspace RDMA):"
  gdscheck -p 2>/dev/null | grep -E "WekaFS|Userspace RDMA" || true
else
  # Try common CUDA path if not in PATH
  if [ -x /usr/local/cuda/gds/tools/gdscheck ]; then
    echo ">> /usr/local/cuda/gds/tools/gdscheck -p (grep WekaFS / Userspace RDMA):"
    /usr/local/cuda/gds/tools/gdscheck -p 2>/dev/null | grep -E "WekaFS|Userspace RDMA" || true
  elif [ -x /usr/local/cuda-12.6/gds/tools/gdscheck ]; then
    echo ">> /usr/local/cuda-12.6/gds/tools/gdscheck -p (grep WekaFS / Userspace RDMA):"
    /usr/local/cuda-12.6/gds/tools/gdscheck -p 2>/dev/null | grep -E "WekaFS|Userspace RDMA" || true
  else
    check 1 "gdscheck not found in PATH or common CUDA locations"
  fi
fi
echo

########################
# 10) WEKA mount line
########################
echo ">> WEKA mount (for reference):"
mount | grep -E ' /mnt/weka ' || echo "No /mnt/weka mount found"
echo

echo "===== SUMMARY ====="
if [ "$fail_count" -eq 0 ]; then
  echo -e "${GREEN}All expected tunings appear to be in place.${NC}"
else
  echo -e "${RED}$fail_count check(s) FAILED.${NC} Review the [FAIL] lines above and adjust the system."
fi

