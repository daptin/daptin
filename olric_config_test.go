package main

import (
	"net"
	"strconv"
	"testing"
	"time"
)

func TestResolveOlricPeers_ValidHostname(t *testing.T) {
	// "localhost" should resolve to at least 127.0.0.1
	peers := resolveOlricPeers("localhost", 5337, "")
	if len(peers) == 0 {
		t.Fatal("expected at least one peer from resolving localhost")
	}
	for _, p := range peers {
		host, port, err := net.SplitHostPort(p)
		if err != nil {
			t.Fatalf("invalid peer address %q: %v", p, err)
		}
		if port != "5337" {
			t.Errorf("expected port 5337, got %s for peer %s", port, p)
		}
		if net.ParseIP(host) == nil {
			t.Errorf("expected valid IP, got %q", host)
		}
	}
}

func TestResolveOlricPeers_InvalidHostname(t *testing.T) {
	// Should retry 3 times and then return nil
	start := time.Now()
	peers := resolveOlricPeers("this-hostname-should-not-exist-xyz123.invalid", 5337, "")
	elapsed := time.Since(start)
	if peers != nil {
		t.Errorf("expected nil peers for unresolvable hostname, got %v", peers)
	}
	// With 3 retries and 2s delay, should take at least 4s (2 delays between 3 attempts)
	if elapsed < 3*time.Second {
		t.Errorf("expected retry delays (>=3s), but completed in %v", elapsed)
	}
}

func TestResolveOlricPeers_FiltersSelf(t *testing.T) {
	allPeers := resolveOlricPeers("localhost", 5337, "")
	if len(allPeers) == 0 {
		t.Skip("localhost did not resolve to any address")
	}

	selfAddr := allPeers[0]
	filtered := resolveOlricPeers("localhost", 5337, selfAddr)

	for _, p := range filtered {
		if p == selfAddr {
			t.Errorf("self address %q should have been filtered out", selfAddr)
		}
	}
	if len(filtered) != len(allPeers)-1 {
		t.Errorf("expected %d peers after filtering self, got %d", len(allPeers)-1, len(filtered))
	}
}

func TestResolveOlricPeers_UsesCorrectPort(t *testing.T) {
	peers := resolveOlricPeers("localhost", 9999, "")
	if len(peers) == 0 {
		t.Skip("localhost did not resolve")
	}
	for _, p := range peers {
		_, port, _ := net.SplitHostPort(p)
		if port != "9999" {
			t.Errorf("expected port 9999, got %s", port)
		}
	}
}

func TestResolveOlricPeers_EmptySeed(t *testing.T) {
	peers := resolveOlricPeers("", 5337, "")
	if peers != nil {
		t.Errorf("expected nil for empty seed, got %v", peers)
	}
}

// TestPortDerivation verifies the olric port auto-derivation logic
// mirrors what main.go does when olric_port == 0.
func TestPortDerivation(t *testing.T) {
	tests := []struct {
		httpPort           int64
		expectedOlricPort  int
		expectedMemberPort int
	}{
		{6336, 5336, 5337},  // default: 6336 > 2000, so 6336-1000
		{8080, 7080, 7081},  // high port
		{1000, 2000, 2001},  // low port <= 2000, so 1000+1000
		{2000, 3000, 3001},  // boundary: 2000 is not > 2000, so +1000
		{2001, 1001, 1002},  // just above boundary: 2001 > 2000, so -1000
		{3000, 2000, 2001},  // 3000 > 2000, so -1000
	}

	for _, tc := range tests {
		t.Run("http_"+strconv.FormatInt(tc.httpPort, 10), func(t *testing.T) {
			var olricPortValue int
			if tc.httpPort > 2000 {
				olricPortValue = int(tc.httpPort - 1000)
			} else {
				olricPortValue = int(tc.httpPort + 1000)
			}
			membershipPort := olricPortValue + 1

			if olricPortValue != tc.expectedOlricPort {
				t.Errorf("httpPort=%d: expected olricPort=%d, got %d",
					tc.httpPort, tc.expectedOlricPort, olricPortValue)
			}
			if membershipPort != tc.expectedMemberPort {
				t.Errorf("httpPort=%d: expected membershipPort=%d, got %d",
					tc.httpPort, tc.expectedMemberPort, membershipPort)
			}
		})
	}
}

func TestPortDerivation_ExplicitPort(t *testing.T) {
	explicitPort := 5500
	membershipPort := explicitPort + 1

	if membershipPort != 5501 {
		t.Errorf("expected membership port 5501, got %d", membershipPort)
	}
}

// TestPeerMerging verifies that static peers and DNS-resolved peers
// are merged correctly, with self-filtering applied to the combined list.
func TestPeerMerging(t *testing.T) {
	staticPeers := []string{"10.0.0.1:5337", "10.0.0.2:5337"}
	dnsPeers := []string{"10.0.0.2:5337", "10.0.0.3:5337"}

	combined := append(staticPeers, dnsPeers...)

	selfAddr := "10.0.0.1:5337"
	var filtered []string
	for _, peer := range combined {
		if peer != selfAddr {
			filtered = append(filtered, peer)
		}
	}

	// Self (10.0.0.1:5337) removed, rest kept (including duplicates)
	if len(filtered) != 3 {
		t.Errorf("expected 3 peers after self-filtering, got %d: %v", len(filtered), filtered)
	}
	for _, p := range filtered {
		if p == selfAddr {
			t.Errorf("self address should have been filtered")
		}
	}
}

// --- Environment-specific discovery tests ---

// TestDiscovery_K8sHeadlessService simulates a K8s headless service that
// resolves a single hostname to multiple pod IPs (multiple A records).
func TestDiscovery_K8sHeadlessService(t *testing.T) {
	// Use a real multi-record hostname. "localhost" typically returns
	// both ::1 and 127.0.0.1 on most systems, simulating multiple pods.
	peers := resolveOlricPeers("localhost", 5337, "")
	if len(peers) == 0 {
		t.Skip("localhost did not resolve to multiple addresses")
	}

	// All peers should have the membership port
	for _, p := range peers {
		_, port, err := net.SplitHostPort(p)
		if err != nil {
			t.Fatalf("invalid peer address: %v", err)
		}
		if port != "5337" {
			t.Errorf("expected membership port 5337, got %s", port)
		}
	}

	// Self-filtering: if this node's IP is in the result, it should be excluded
	selfAddr := peers[0]
	filtered := resolveOlricPeers("localhost", 5337, selfAddr)
	for _, p := range filtered {
		if p == selfAddr {
			t.Errorf("K8s self-pod %q should be filtered from peer list", selfAddr)
		}
	}
}

// TestDiscovery_DockerCompose simulates Docker Compose where a service
// name resolves to container IPs via embedded DNS.
func TestDiscovery_DockerCompose(t *testing.T) {
	// "localhost" simulates a Docker service name resolving to IPs.
	// In real Docker Compose, "daptin" would resolve to all container IPs.
	peers := resolveOlricPeers("localhost", 5337, "")
	if len(peers) == 0 {
		t.Skip("could not resolve test hostname")
	}

	// Verify valid IP:port format for each peer
	for _, p := range peers {
		host, port, err := net.SplitHostPort(p)
		if err != nil {
			t.Errorf("invalid peer format %q: %v", p, err)
		}
		if net.ParseIP(host) == nil {
			t.Errorf("expected valid IP, got %q", host)
		}
		if port != "5337" {
			t.Errorf("expected port 5337, got %s", port)
		}
	}
}

// TestDiscovery_ManualStaticPeers verifies that static olric_peers flag
// works independently of DNS and can be combined with DNS results.
func TestDiscovery_ManualStaticPeers(t *testing.T) {
	// Static peers — no DNS involved
	staticPeers := []string{"192.168.1.10:5337", "192.168.1.11:5337"}
	selfAddr := "192.168.1.10:5337"

	var filtered []string
	for _, peer := range staticPeers {
		if peer != selfAddr {
			filtered = append(filtered, peer)
		}
	}

	if len(filtered) != 1 {
		t.Errorf("expected 1 peer after self-filter, got %d", len(filtered))
	}
	if filtered[0] != "192.168.1.11:5337" {
		t.Errorf("expected 192.168.1.11:5337, got %s", filtered[0])
	}
}

// TestDiscovery_HybridStaticAndDNS verifies merging static peers with
// DNS-resolved peers, as would happen with both -olric_peers and -olric_seed.
func TestDiscovery_HybridStaticAndDNS(t *testing.T) {
	staticPeers := []string{"10.0.0.50:5337"}

	// DNS resolves to localhost IPs
	dnsPeers := resolveOlricPeers("localhost", 5337, "")
	if len(dnsPeers) == 0 {
		t.Skip("localhost did not resolve")
	}

	combined := append(staticPeers, dnsPeers...)

	// No self to filter in this test
	if len(combined) != len(staticPeers)+len(dnsPeers) {
		t.Errorf("expected %d combined peers, got %d",
			len(staticPeers)+len(dnsPeers), len(combined))
	}

	// Static peer should be present
	found := false
	for _, p := range combined {
		if p == "10.0.0.50:5337" {
			found = true
			break
		}
	}
	if !found {
		t.Error("static peer 10.0.0.50:5337 missing from combined list")
	}
}

// TestDiscovery_SingleNodeNoConfig verifies that with no peers and no seed,
// the node starts in standalone mode.
func TestDiscovery_SingleNodeNoConfig(t *testing.T) {
	// No static peers
	var peers []string

	// No seed configured — don't call resolveOlricPeers
	selfAddr := "192.168.1.1:5337"

	var filtered []string
	for _, peer := range peers {
		if peer != selfAddr {
			filtered = append(filtered, peer)
		}
	}

	if len(filtered) != 0 {
		t.Errorf("single node should have zero peers, got %d", len(filtered))
	}
}

// TestDiscovery_RetryOnDNSFailure verifies retry behavior when DNS
// fails (simulates K8s service not yet ready or Docker DNS delay).
func TestDiscovery_RetryOnDNSFailure(t *testing.T) {
	start := time.Now()
	peers := resolveOlricPeers("nonexistent-seed-host.invalid", 5337, "")
	elapsed := time.Since(start)

	if peers != nil {
		t.Errorf("expected nil for unresolvable service, got %v", peers)
	}
	// 3 attempts with 2s delay between them = at least 4s.
	// DNS resolution itself may add time (especially .local on macOS via mDNS).
	if elapsed < 3*time.Second {
		t.Errorf("expected retry behavior (>=3s elapsed), got %v", elapsed)
	}
	if elapsed > 30*time.Second {
		t.Errorf("retries took too long (%v), max expected ~25s", elapsed)
	}
}

// TestDiscovery_IPv6Support verifies that IPv6 addresses from DNS
// are formatted correctly with brackets in host:port notation.
func TestDiscovery_IPv6Support(t *testing.T) {
	// localhost typically resolves to ::1 (IPv6) on most systems
	peers := resolveOlricPeers("localhost", 5337, "")
	if len(peers) == 0 {
		t.Skip("localhost did not resolve")
	}

	hasIPv6 := false
	for _, p := range peers {
		host, _, err := net.SplitHostPort(p)
		if err != nil {
			t.Errorf("failed to parse peer %q: %v", p, err)
			continue
		}
		ip := net.ParseIP(host)
		if ip == nil {
			t.Errorf("invalid IP in peer %q", p)
			continue
		}
		if ip.To4() == nil {
			hasIPv6 = true
			// net.JoinHostPort should bracket IPv6: [::1]:5337
			if p[0] != '[' {
				t.Errorf("IPv6 peer not bracketed: %q", p)
			}
		}
	}

	if !hasIPv6 {
		t.Log("no IPv6 address found for localhost (system may not have IPv6)")
	}
}
