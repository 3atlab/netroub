package network

import (
	"crypto/tls"
	"crypto/x509"

	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/3atlab/netroub/pkg/model"
	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// calculateSubnetSize determines the minimum subnet prefix length for the given device count.
// Returns prefix length (e.g., 24 for /24) and number of usable IPs.
func calculateSubnetSize(deviceCount int) (int, int) {
	// Find minimum prefix that provides enough usable IPs for devices + gateway
	// usable = total - 2 (network address and broadcast address are excluded)
	// required = deviceCount + 1 (for Docker/containerlab gateway)
	required := deviceCount + 1
	for prefix := 30; prefix >= 16; prefix-- {
		usable := (1 << (32 - prefix)) - 2
		if usable >= required {
			return prefix, usable
		}
	}
	return 16, (1 << 16) - 2 // Maximum /16
}

// extractLabIndex extracts the numeric index from a lab name (e.g., "baseline_001" -> 1).
// Returns 0 if no number is found.
func extractLabIndex(labName string) int {
	// Find the last underscore and parse the number after it
	idx := strings.LastIndex(labName, "_")
	if idx >= 0 && idx < len(labName)-1 {
		numStr := labName[idx+1:]
		var num int
		fmt.Sscanf(numStr, "%d", &num)
		return num
	}
	return 0
}

// generateSubnet generates a unique IPv4 subnet within 172.16.0.0/12 range.
// The subnet size is determined by deviceCount, and the specific subnet
// is determined by labIndex.
// Returns the subnet string and an error if the allocation exceeds the available range.
func generateSubnet(labName string, deviceCount int) (string, error) {
	prefix, _ := calculateSubnetSize(deviceCount)
	labIndex := extractLabIndex(labName)

	// 172.16.0.0/12 range: 172.16.0.0 - 172.31.255.255
	// Base: 172.16.0.0 = 0xAC100000
	baseIP := uint32(0xAC100000) // 172.16.0.0

	// Calculate subnet size in IPs
	subnetSize := uint32(1 << (32 - prefix))

	// Calculate offset for this lab
	offset := uint32(labIndex) * subnetSize

	// Check if we exceed 172.16.0.0/12 range (172.16.0.0 - 172.31.255.255)
	// 172.31.255.255 = 0xAC1FFFFF
	maxIP := uint32(0xAC1FFFFF)
	subnetIP := baseIP + offset

	if subnetIP+subnetSize-1 > maxIP {
		return "", fmt.Errorf("subnet allocation exceeds 172.16.0.0/12 range: lab index %d with /%d subnets requires more address space (device count: %d)", labIndex, prefix, deviceCount)
	}

	// Convert to dotted notation
	a := (subnetIP >> 24) & 0xFF
	b := (subnetIP >> 16) & 0xFF
	c := (subnetIP >> 8) & 0xFF
	d := subnetIP & 0xFF

	return fmt.Sprintf("%d.%d.%d.%d/%d", a, b, c, d, prefix), nil
}

// generateIPv6Subnet generates a unique IPv6 subnet for parallel execution.
// Uses the format 3fff:172:20:{labIndex}::/64 to avoid collisions.
// Returns the subnet string and an error if labIndex exceeds 65535.
func generateIPv6Subnet(labName string) (string, error) {
	labIndex := extractLabIndex(labName)

	// Each lab gets a /64 subnet within 3fff:172:20::/48
	// The 4th segment (16 bits) is determined by labIndex
	if labIndex > 65535 {
		return "", fmt.Errorf("lab index %d exceeds maximum for IPv6 subnet allocation (max 65535)", labIndex)
	}

	return fmt.Sprintf("3fff:172:20:%x::/64", labIndex), nil
}

func EmulateNetwork() error {
	labName := model.GetLabName()

	// Get device count for subnet size calculation
	deviceCount := len(model.Devices.Nodes)
	if deviceCount == 0 {
		deviceCount = 254 // Default to /24 if no devices loaded
	}

	// Generate unique IPv4 subnet based on device count and lab index
	ipv4Subnet, err := generateSubnet(labName, deviceCount)
	if err != nil {
		return fmt.Errorf("failed to allocate IPv4 subnet: %w", err)
	}

	// Generate unique IPv6 subnet for parallel execution
	ipv6Subnet, err := generateIPv6Subnet(labName)
	if err != nil {
		return fmt.Errorf("failed to allocate IPv6 subnet: %w", err)
	}

	// Serialize containerlab deploy to avoid netlink race conditions
	networkOpMu.Lock()
	defer networkOpMu.Unlock()

	// Log after acquiring lock so log order reflects actual execution order
	logrus.Infof("Deploying network with lab name: %s", labName)

	// Use unique network name for parallel execution
	networkName := "clab-" + labName
	cmd := exec.Command("sudo", "containerlab", "deploy",
		"--name", labName,
		"--topo", model.Scenar.Topo,
		"--network", networkName,
		"--ipv4-subnet", ipv4Subnet,
		"--ipv6-subnet", ipv6Subnet)
	out, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			errMsg := string(exitError.Stderr)
			log.Fatal(errMsg)
		} else {
			log.Fatal(err.Error())
		}
	}
	fmt.Println(string(out))
	return nil
}

func DestroyNetwork() error {
	labName := model.GetLabName()

	// Serialize containerlab destroy to avoid netlink race conditions
	networkOpMu.Lock()
	defer networkOpMu.Unlock()

	// Log after acquiring lock so log order reflects actual execution order
	logrus.Infof("Destroying network with lab name: %s", labName)

	// Use --name only (without --topo) to avoid containerlab trying to
	// create a clab instance with default network settings.
	// --cleanup ensures Docker network is also removed.
	cmd := exec.Command("sudo", "containerlab", "destroy",
		"--name", labName,
		"--cleanup")
	out, err := cmd.Output()
	if err != nil {
		fmt.Println("Error while destroy the emulated network")
		return err
	}
	fmt.Println(string(out))

	return nil
}

func CreateDockerClient(c *cli.Context) error {
	tlsCfg, err := tlsConfig(c)
	if err != nil {
		return err
	}
	chaos.DockerClient, err = container.NewClient("unix:///var/run/docker.sock", tlsCfg)
	if err != nil {
		return errors.Wrap(err, "could not create Docker client")
	}
	return nil
}

// tlsConfig translates the command-line options into a tls.Config struct
func tlsConfig(c *cli.Context) (*tls.Config, error) {
	var tlsCfg *tls.Config
	var err error
	caCertFlag := c.GlobalString("tlscacert")
	certFlag := c.GlobalString("tlscert")
	keyFlag := c.GlobalString("tlskey")

	if c.GlobalBool("tls") || c.GlobalBool("tlsverify") {
		tlsCfg = &tls.Config{
			InsecureSkipVerify: !c.GlobalBool("tlsverify"), //nolint:gosec
		}

		// Load CA cert
		if caCertFlag != "" {
			var caCert []byte
			if strings.HasPrefix(caCertFlag, "/") {
				caCert, err = os.ReadFile(caCertFlag)
				if err != nil {
					return nil, errors.Wrap(err, "unable to read CA certificate")
				}
			} else {
				caCert = []byte(caCertFlag)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsCfg.RootCAs = caCertPool
		}

		// Load client certificate
		if certFlag != "" && keyFlag != "" {
			var cert tls.Certificate
			if strings.HasPrefix(certFlag, "/") && strings.HasPrefix(keyFlag, "/") {
				cert, err = tls.LoadX509KeyPair(certFlag, keyFlag)
				if err != nil {
					return nil, errors.Wrap(err, "unable to load client certificate")
				}
			} else {
				cert, err = tls.X509KeyPair([]byte(certFlag), []byte(keyFlag))
				if err != nil {
					return nil, errors.Wrap(err, "unable to load client certificate")
				}
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
	}
	return tlsCfg, nil
}
