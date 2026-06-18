package resource

import (
	"bytes"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/artpar/go-guerrilla"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

func BuildSMTPServerConfigs(servers []map[string]interface{}, certificateManager *CertificateManager, transaction *sqlx.Tx) ([]guerrilla.ServerConfig, []string, error) {
	serverConfig := make([]guerrilla.ServerConfig, 0, len(servers))
	hosts := make([]string, 0, len(servers))

	tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), "daptin-certs")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp directory for SMTP certs: %w", err)
	}

	var configErrors []string
	for _, server := range servers {
		maxSize, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_size"]), 10, 32)
		maxClients, _ := strconv.ParseInt(fmt.Sprintf("%v", server["max_clients"]), 10, 32)

		hostnameVal, ok := server["hostname"].(string)
		if !ok || hostnameVal == "" {
			log.Printf("Skipping SMTP server entry with missing hostname")
			continue
		}
		hostname := hostnameVal

		cert, err := certificateManager.GetTLSConfig(hostname, true, transaction)
		if err != nil {
			msg := fmt.Sprintf("failed to generate certificates for SMTP server %s: %v", hostname, err)
			log.Print(msg)
			configErrors = append(configErrors, msg)
			continue
		}

		privateKeyFilePath := filepath.Join(tempDirectoryPath, hostname+".private.cert.pem")
		publicKeyFilePath := filepath.Join(tempDirectoryPath, hostname+".public.cert.pem")
		rootCaFile := filepath.Join(tempDirectoryPath, hostname+".root.cert.pem")

		if err := os.WriteFile(publicKeyFilePath, smtpCertificateChainPEM(cert.CertPEM, cert.RootCert), 0600); err != nil {
			msg := fmt.Sprintf("failed to write certificate chain for SMTP server %s: %v", hostname, err)
			log.Print(msg)
			configErrors = append(configErrors, msg)
			continue
		}
		if err := os.WriteFile(rootCaFile, cert.RootCert, 0600); err != nil {
			msg := fmt.Sprintf("failed to write root certificate for SMTP server %s: %v", hostname, err)
			log.Print(msg)
			configErrors = append(configErrors, msg)
			continue
		}
		if err := os.WriteFile(privateKeyFilePath, cert.PrivatePEMDecrypted, 0600); err != nil {
			msg := fmt.Sprintf("failed to write private key for SMTP server %s: %v", hostname, err)
			log.Print(msg)
			configErrors = append(configErrors, msg)
			continue
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

		hosts = append(hosts, hostname)
		serverConfig = append(serverConfig, config)
	}

	if len(serverConfig) == 0 && len(configErrors) > 0 {
		return nil, hosts, fmt.Errorf("failed to build SMTP server configs: %s", strings.Join(configErrors, "; "))
	}
	if len(configErrors) > 0 {
		log.Warnf("Built SMTP config with skipped server entries: %s", strings.Join(configErrors, "; "))
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

func smtpCertificateChainPEM(certPEM, rootCertPEM []byte) []byte {
	var out bytes.Buffer
	seen := make(map[string]bool)
	appendCertificateBlocks(&out, seen, certPEM)
	appendCertificateBlocks(&out, seen, rootCertPEM)
	return out.Bytes()
}

func appendCertificateBlocks(out *bytes.Buffer, seen map[string]bool, raw []byte) {
	rest := bytes.TrimSpace(raw)
	for len(rest) > 0 {
		block, remaining := pem.Decode(rest)
		if block == nil {
			return
		}
		if block.Type == "CERTIFICATE" {
			key := string(block.Bytes)
			if !seen[key] {
				_ = pem.Encode(out, block)
				seen[key] = true
			}
		}
		rest = bytes.TrimSpace(remaining)
	}
}
