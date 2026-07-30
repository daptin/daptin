package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/artpar/go-guerrilla"
	"github.com/jmoiron/sqlx"
)

func BuildSMTPServerConfigs(servers []map[string]interface{}, certificateManager *CertificateManager, transaction *sqlx.Tx) ([]guerrilla.ServerConfig, []string, error) {
	serverConfig := make([]guerrilla.ServerConfig, 0, len(servers))
	hosts := make([]string, 0, len(servers))
	seenHosts := make(map[string]struct{}, len(servers))
	seenListeners := make(map[string]string)

	// Recipient domains are independent of SMTP listeners. Disabled mail_server
	// rows intentionally remain valid recipient domains, but must not require TLS
	// material or participate in listener reloads.
	for _, server := range servers {
		hostname, ok := server["hostname"].(string)
		hostname = strings.ToLower(strings.TrimSpace(hostname))
		if !ok || hostname == "" {
			return nil, nil, fmt.Errorf("SMTP server entry has a missing hostname")
		}
		if _, exists := seenHosts[hostname]; !exists {
			hosts = append(hosts, hostname)
			seenHosts[hostname] = struct{}{}
		}

		if !smtpConfigBool(server["is_enabled"]) {
			continue
		}
		listenInterface := strings.TrimSpace(fmt.Sprintf("%v", server["listen_interface"]))
		if listenInterface == "" || listenInterface == "<nil>" {
			return nil, nil, fmt.Errorf("enabled SMTP server %s has a missing listen_interface", hostname)
		}
		if existingHost, exists := seenListeners[listenInterface]; exists {
			return nil, nil, fmt.Errorf("enabled SMTP servers %s and %s share listen_interface %s", existingHost, hostname, listenInterface)
		}
		seenListeners[listenInterface] = hostname
	}
	if len(seenListeners) == 0 {
		return serverConfig, hosts, nil
	}

	tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), "daptin-certs")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp directory for SMTP certs: %w", err)
	}

	for _, server := range servers {
		if !smtpConfigBool(server["is_enabled"]) {
			continue
		}
		maxSize, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_size"]), 10, 32)
		maxClients, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_clients"]), 10, 32)

		hostname := strings.ToLower(strings.TrimSpace(server["hostname"].(string)))

		cert, err := certificateManager.GetTLSConfig(hostname, true, transaction)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate certificates for SMTP server %s: %w", hostname, err)
		}

		privateKeyFilePath := filepath.Join(tempDirectoryPath, hostname+".private.cert.pem")
		publicKeyFilePath := filepath.Join(tempDirectoryPath, hostname+".public.cert.pem")
		rootCaFile := filepath.Join(tempDirectoryPath, hostname+".root.cert.pem")

		if err := os.WriteFile(publicKeyFilePath, certificateChainPEM(cert.CertPEM, cert.RootCert), 0600); err != nil {
			return nil, nil, fmt.Errorf("failed to write certificate chain for SMTP server %s: %w", hostname, err)
		}
		if err := os.WriteFile(rootCaFile, cert.RootCert, 0600); err != nil {
			return nil, nil, fmt.Errorf("failed to write root certificate for SMTP server %s: %w", hostname, err)
		}
		if err := os.WriteFile(privateKeyFilePath, cert.PrivatePEMDecrypted, 0600); err != nil {
			return nil, nil, fmt.Errorf("failed to write private key for SMTP server %s: %w", hostname, err)
		}

		config := guerrilla.ServerConfig{
			IsEnabled:       smtpConfigBool(server["is_enabled"]),
			ListenInterface: fmt.Sprintf("%v", server["listen_interface"]),
			Hostname:        hostname,
			MaxSize:         maxSize,
			Timeout:         30,
			TLS: guerrilla.ServerTLSConfig{
				StartTLSOn:               true,
				AlwaysOn:                 smtpConfigBool(server["always_on_tls"]),
				PrivateKeyFile:           privateKeyFilePath,
				PublicKeyFile:            publicKeyFilePath,
				RootCAs:                  rootCaFile,
				ClientAuthType:           "NoClientCert",
				PreferServerCipherSuites: true,
			},
			MaxClients:   int(maxClients),
			XClientOn:    smtpConfigBool(server["xclient_on"]),
			AuthRequired: smtpConfigBool(server["authentication_required"]),
			AuthTypes:    []string{"LOGIN"},
		}

		serverConfig = append(serverConfig, config)
	}

	return serverConfig, hosts, nil
}

func smtpConfigBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int8:
		return v != 0
	case int16:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case uint:
		return v != 0
	case uint8:
		return v != 0
	case uint16:
		return v != 0
	case uint32:
		return v != 0
	case uint64:
		return v != 0
	case string:
		normalized := strings.TrimSpace(strings.ToLower(v))
		return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
	default:
		normalized := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", value)))
		return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
	}
}
