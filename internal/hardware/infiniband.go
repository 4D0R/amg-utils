package hardware

import (
	"fmt"
	"net"
	"strings"
)

// InfiniBandNetworkInterface represents an InfiniBand network interface with its status
type InfiniBandNetworkInterface struct {
	Name      string
	IPAddress string
	Status    string
}

// GetInfiniBandNetworkInterfaces returns InfiniBand network interfaces with their IP addresses and status
func GetInfiniBandNetworkInterfaces() ([]InfiniBandNetworkInterface, error) {
	var ibInterfaces []InfiniBandNetworkInterface

	// Get all network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %v", err)
	}

	// Filter for InfiniBand interfaces (typically named ib0, ib1, etc.)
	for _, iface := range interfaces {
		if strings.HasPrefix(iface.Name, "ib") {
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
