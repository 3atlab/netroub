package network

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateSubnetSize(t *testing.T) {
	// Note: required IPs = deviceCount + 1 (for Docker/containerlab gateway)
	tests := []struct {
		name           string
		deviceCount    int
		expectedPrefix int
		minUsable      int
	}{
		{"single device", 1, 30, 2},            // 1+1=2 required, /30 has 2
		{"two devices", 2, 29, 3},              // 2+1=3 required, /30 only has 2, need /29 (6)
		{"small network", 10, 28, 11},          // 10+1=11 required, /28 has 14
		{"medium network /25", 100, 25, 101},   // 100+1=101 required, /25 has 126
		{"needs /24", 126, 24, 127},            // 126+1=127 required, /25 only has 126, need /24
		{"max /24", 253, 24, 254},              // 253+1=254 required, /24 has 254
		{"needs /23", 254, 23, 255},            // 254+1=255 required, /24 only has 254, need /23
		{"medium /23", 400, 23, 401},           // 400+1=401 required, /23 has 510
		{"max /23", 509, 23, 510},              // 509+1=510 required, /23 has 510
		{"needs /22", 510, 22, 511},            // 510+1=511 required, /23 only has 510, need /22
		{"large network", 1000, 22, 1001},      // 1000+1=1001 required, /22 has 1022
		{"very large network", 2000, 21, 2001}, // 2000+1=2001 required, /21 has 2046
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, usable := calculateSubnetSize(tt.deviceCount)
			assert.Equal(t, tt.expectedPrefix, prefix, "prefix mismatch for %d devices", tt.deviceCount)
			assert.GreaterOrEqual(t, usable, tt.minUsable, "usable IPs should be >= device count")
		})
	}
}

func TestExtractLabIndex(t *testing.T) {
	tests := []struct {
		labName  string
		expected int
	}{
		{"baseline_001", 1},
		{"baseline_010", 10},
		{"baseline_100", 100},
		{"baseline_999", 999},
		{"A1_delay_001", 1},
		{"A1_delay_050", 50},
		{"bgp_features", 0}, // No number suffix
		{"test", 0},         // No underscore
		{"test_", 0},        // Empty after underscore
		{"test_abc", 0},     // Non-numeric suffix
	}

	for _, tt := range tests {
		t.Run(tt.labName, func(t *testing.T) {
			result := extractLabIndex(tt.labName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateSubnet(t *testing.T) {
	tests := []struct {
		name        string
		labName     string
		deviceCount int
		expected    string
		expectError bool
	}{
		{
			name:        "first lab small network",
			labName:     "baseline_001",
			deviceCount: 16,
			expected:    "172.16.0.32/27", // /27 = 32 IPs, index 1 * 32 = offset 32
			expectError: false,
		},
		{
			name:        "second lab small network",
			labName:     "baseline_002",
			deviceCount: 16,
			expected:    "172.16.0.64/27", // /27 = 32 IPs, index 2 * 32 = offset 64
			expectError: false,
		},
		{
			name:        "lab index 0",
			labName:     "bgp_features",
			deviceCount: 16,
			expected:    "172.16.0.0/27", // /27, index 0
			expectError: false,
		},
		{
			name:        "large network needs /23",
			labName:     "baseline_001",
			deviceCount: 300,
			expected:    "172.16.2.0/23", // /23 = 512 IPs, index 1 * 512 = offset 512
			expectError: false,
		},
		{
			name:        "very large network needs /22",
			labName:     "baseline_001",
			deviceCount: 600,
			expected:    "172.16.4.0/22", // /22 = 1024 IPs, index 1 * 1024 = offset 1024
			expectError: false,
		},
		{
			name:        "medium network /24",
			labName:     "baseline_005",
			deviceCount: 200,
			expected:    "172.16.5.0/24", // /24 = 256 IPs, index 5 * 256 = offset 1280
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateSubnet(tt.labName, tt.deviceCount)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGenerateSubnet_RangeExceeded(t *testing.T) {
	// 172.16.0.0/12 range has 2^20 = 1,048,576 IPs (172.16.0.0 - 172.31.255.255)
	// With /24 subnets (256 IPs each), we can have 4096 subnets (index 0-4095)
	// Lab index 4096 with /24 should exceed the range

	// For 200 devices, we need /24 (256 IPs per subnet)
	// Index 4096 * 256 = 1,048,576 which would start at 172.32.0.0, exceeding the range
	_, err := generateSubnet("baseline_4096", 200)
	if assert.Error(t, err, "should error when exceeding 172.16.0.0/12 range") {
		assert.Contains(t, err.Error(), "exceeds 172.16.0.0/12 range")
	}
}

func TestGenerateSubnet_ParallelExecution(t *testing.T) {
	// Verify that parallel labs get unique, non-overlapping subnets
	subnets := make(map[string]bool)

	for i := 1; i <= 100; i++ {
		labName := fmt.Sprintf("baseline_%03d", i)
		subnet, err := generateSubnet(labName, 16)
		assert.NoError(t, err, "lab %s should not error", labName)

		assert.False(t, subnets[subnet], "subnet %s should be unique (lab %s)", subnet, labName)
		subnets[subnet] = true
	}
}

func TestGenerateIPv6Subnet(t *testing.T) {
	tests := []struct {
		name     string
		labName  string
		expected string
	}{
		{
			name:     "lab index 0",
			labName:  "bgp_features",
			expected: "3fff:172:20:0::/64",
		},
		{
			name:     "lab index 1",
			labName:  "baseline_001",
			expected: "3fff:172:20:1::/64",
		},
		{
			name:     "lab index 10",
			labName:  "baseline_010",
			expected: "3fff:172:20:a::/64",
		},
		{
			name:     "lab index 255",
			labName:  "baseline_255",
			expected: "3fff:172:20:ff::/64",
		},
		{
			name:     "lab index 256",
			labName:  "baseline_256",
			expected: "3fff:172:20:100::/64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateIPv6Subnet(tt.labName)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateIPv6Subnet_ParallelExecution(t *testing.T) {
	// Verify that parallel labs get unique IPv6 subnets
	subnets := make(map[string]bool)

	for i := 1; i <= 100; i++ {
		labName := fmt.Sprintf("baseline_%03d", i)
		subnet, err := generateIPv6Subnet(labName)
		assert.NoError(t, err, "lab %s should not error", labName)

		assert.False(t, subnets[subnet], "IPv6 subnet %s should be unique (lab %s)", subnet, labName)
		subnets[subnet] = true
	}
}
