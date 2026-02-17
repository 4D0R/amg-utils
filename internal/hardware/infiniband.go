package hardware

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// InfiniBandNetworkInterface represents an InfiniBand network interface with its status
type InfiniBandNetworkInterface struct {
	Name      string
	IPAddress string
	Status    string
}

// isPrimaryPort checks if a network interface is on PCI function .0 (primary port).
// For dual-port NICs, both ports share the same PCIe bandwidth, so we only use
// the primary port (.0) to avoid contention and optimize load balancing.
// Returns true if this is a primary port, false otherwise or if unable to determine.
func isPrimaryPort(ifaceName string) bool {
	// Read the symlink to get the PCI device path
	devicePath := fmt.Sprintf("/sys/class/net/%s/device", ifaceName)
	pciPath, err := os.Readlink(devicePath)
	if err != nil {
		// If we can't determine, include it (fail-safe for non-PCI devices)
		return true
	}

	// Extract the PCI address (last component of path, e.g., "0000:0c:00.0")
	pciDev := filepath.Base(pciPath)

	// Check if it's PCI function .0 (primary port of a multi-function device)
	// Format: DDDD:BB:DD.F where F is the function number
	// We want function 0 (e.g., "0000:0c:00.0" is primary, "0000:0c:00.1" is secondary)
	return strings.HasSuffix(pciDev, ".0")
}

// GetInfiniBandNetworkInterfaces returns InfiniBand network interfaces with their IP addresses and status
func GetInfiniBandNetworkInterfaces() ([]InfiniBandNetworkInterface, error) {
	var ibInterfaces []InfiniBandNetworkInterface

	// Get all network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %v", err)
	}

	// Filter for RDMA interfaces (typically named ib0, ib1, rdma0, rdma1, etc.)
	for _, iface := range interfaces {
		if strings.HasPrefix(iface.Name, "ib") || strings.HasPrefix(iface.Name, "rdma") {
			// For dual-port NICs, only use the primary port (PCI function .0)
			// to avoid PCIe bandwidth contention and optimize load balancing
			if !isPrimaryPort(iface.Name) {
				continue
			}

			// Get IP addresses for this interface
			addrs, err := iface.Addrs()
			if err != nil {
				continue // Skip this interface if we can't get addresses
			}

			// Determine interface status
			status := "down"
			if iface.Flags&net.FlagUp != 0 {
				status = "up"
			}

			// If no IP addresses, still show the interface but without IP
			if len(addrs) == 0 {
				ibInterfaces = append(ibInterfaces, InfiniBandNetworkInterface{
					Name:      iface.Name,
					IPAddress: "no IP assigned",
					Status:    status,
				})
				continue
			}

			// Add interface for each IP address
			for _, addr := range addrs {
				// Parse the IP address from CIDR notation
				ip, _, err := net.ParseCIDR(addr.String())
				if err != nil {
					// If not CIDR, try to parse as IP directly
					ip = net.ParseIP(addr.String())
					if ip == nil {
						continue
					}
				}

				// Only include IPv4 and IPv6 addresses (skip link-local etc)
				if ip.To4() != nil || (ip.To16() != nil && !ip.IsLinkLocalUnicast()) {
					ibInterfaces = append(ibInterfaces, InfiniBandNetworkInterface{
						Name:      iface.Name,
						IPAddress: addr.String(),
						Status:    status,
					})
				}
			}
		}
	}

	return ibInterfaces, nil
}
