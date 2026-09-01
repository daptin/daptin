package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/catalog"
	"github.com/daptin/llmgateway/contract"
	"github.com/doug-martin/goqu/v9"
)

type daptinCatalog struct {
	cruds       map[string]*resource.DbResource
	mu          sync.Mutex
	revision    uint64
	fingerprint [sha256.Size]byte
}

func (source *daptinCatalog) Load(ctx context.Context, after uint64) (catalog.Document, error) {
	if err := ctx.Err(); err != nil {
		return catalog.Document{}, err
	}
	transaction, err := source.cruds["world"].Connection().Beginx()
	if err != nil {
		return catalog.Document{}, fmt.Errorf("begin LLM catalog read: %w", err)
	}
	defer transaction.Rollback()

	providers, err := source.cruds["llm_provider"].GetActiveLLMProviders(transaction)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("load LLM providers: %w", err)
	}
	modelRows, _, err := source.cruds["llm_model"].GetRowsByWhereClauseWithTransaction(
		"llm_model", nil, transaction, goqu.Ex{"enable": true},
	)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("load LLM models: %w", err)
	}
	deploymentRows, _, err := source.cruds["llm_deployment"].GetRowsByWhereClauseWithTransaction(
		"llm_deployment", map[string]bool{"llm_model": true, "llm_provider": true}, transaction, goqu.Ex{"enable": true},
	)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("load LLM deployments: %w", err)
	}

	document := catalog.Document{
		Providers:   make([]catalog.Provider, 0, len(providers)),
		Models:      make([]catalog.Model, 0, len(modelRows)),
		Deployments: make([]catalog.Deployment, 0, len(deploymentRows)),
	}
	credentialReferences := make(map[string]bool, len(providers))
	for _, provider := range providers {
		parameters, marshalErr := json.Marshal(provider.ProviderParameters)
		if marshalErr != nil {
			return catalog.Document{}, fmt.Errorf("encode LLM provider %q parameters: %w", provider.Name, marshalErr)
		}
		secretReference := ""
		if provider.CredentialId != daptinid.NullReferenceId {
			secretReference = provider.CredentialId.String()
			credentialReferences[secretReference] = true
		}
		document.Providers = append(document.Providers, catalog.Provider{
			ID: contract.ID(provider.ReferenceId.String()), Name: provider.Name, Type: provider.ProviderType,
			BaseURL: provider.BaseUrl, AllowInsecure: provider.AllowInsecure, AllowPrivateNetwork: provider.AllowPrivateNetwork,
			SecretRef: secretReference, Parameters: parameters, Enabled: provider.Enable,
		})
	}

	fallbacks := make(map[contract.ID][]string, len(modelRows))
	modelIDsByName := make(map[string]contract.ID, len(modelRows))
	for _, row := range modelRows {
		modelID := contract.ID(daptinid.InterfaceToDIR(row["reference_id"]).String())
		name := strings.TrimSpace(resource.StringOrEmpty(row["name"]))
		var operations []contract.Operation
		if err := json.Unmarshal([]byte(resource.StringOrEmpty(row["operations"])), &operations); err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM model %q operations: %w", name, err)
		}
		capabilities := make(map[string]bool)
		if err := json.Unmarshal([]byte(resource.StringOrEmpty(row["capabilities"])), &capabilities); err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM model %q capabilities: %w", name, err)
		}
		var fallbackNames []string
		if err := json.Unmarshal([]byte(resource.StringOrEmpty(row["fallback_models"])), &fallbackNames); err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM model %q fallbacks: %w", name, err)
		}
		model := catalog.Model{
			ID: modelID, Name: name, Operations: operations, Capabilities: capabilities,
			RoutingStrategy:            resource.StringOrEmpty(row["routing_strategy"]),
			DefaultParameters:          []byte(resource.StringOrEmpty(row["default_parameters"])),
			UnsupportedParameterPolicy: resource.StringOrEmpty(row["unsupported_parameter_policy"]), Enabled: true,
		}
		document.Models = append(document.Models, model)
		modelIDsByName[name] = modelID
		fallbacks[modelID] = fallbackNames
	}
	for index := range document.Models {
		for _, fallbackName := range fallbacks[document.Models[index].ID] {
			fallbackID, ok := modelIDsByName[fallbackName]
			if !ok {
				return catalog.Document{}, fmt.Errorf("LLM model %q references unknown fallback %q", document.Models[index].Name, fallbackName)
			}
			document.Models[index].FallbackModelIDs = append(document.Models[index].FallbackModelIDs, fallbackID)
		}
	}

	for _, row := range deploymentRows {
		name := strings.TrimSpace(resource.StringOrEmpty(row["name"]))
		priority, err := resource.ResourceRowInt64(row["priority"])
		if err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM deployment %q priority: %w", name, err)
		}
		weight, err := resource.ResourceRowInt64(row["weight"])
		if err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM deployment %q weight: %w", name, err)
		}
		requestTimeoutMS, err := resource.ResourceRowInt64(row["request_timeout_ms"])
		if err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM deployment %q request timeout: %w", name, err)
		}
		connectTimeoutMS, err := resource.ResourceRowInt64(row["connect_timeout_ms"])
		if err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM deployment %q connect timeout: %w", name, err)
		}
		maxConcurrency, err := resource.ResourceRowInt64(row["max_concurrency"])
		if err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM deployment %q max concurrency: %w", name, err)
		}
		rpm, err := resource.ResourceRowInt64(row["rpm"])
		if err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM deployment %q rpm: %w", name, err)
		}
		tpm, err := resource.ResourceRowInt64(row["tpm"])
		if err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM deployment %q tpm: %w", name, err)
		}
		var operations []contract.Operation
		if err := json.Unmarshal([]byte(resource.StringOrEmpty(row["operations"])), &operations); err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM deployment %q operations: %w", name, err)
		}
		var pricing struct {
			Input      int64 `json:"input_micros_per_million"`
			Output     int64 `json:"output_micros_per_million"`
			CacheRead  int64 `json:"cache_read_micros_per_million"`
			CacheWrite int64 `json:"cache_write_micros_per_million"`
			Reasoning  int64 `json:"reasoning_micros_per_million"`
		}
		if err := decodeCatalogObject(resource.StringOrEmpty(row["pricing"]), &pricing); err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM deployment %q pricing: %w", name, err)
		}
		var health struct {
			Enabled          bool   `json:"enabled"`
			Path             string `json:"path"`
			Model            string `json:"model"`
			IntervalMS       int64  `json:"interval_ms"`
			TimeoutMS        int64  `json:"timeout_ms"`
			FailureThreshold int64  `json:"failure_threshold"`
		}
		if err := decodeCatalogObject(resource.StringOrEmpty(row["health_check"]), &health); err != nil {
			return catalog.Document{}, fmt.Errorf("decode LLM deployment %q health check: %w", name, err)
		}
		document.Deployments = append(document.Deployments, catalog.Deployment{
			ID: contract.ID(daptinid.InterfaceToDIR(row["reference_id"]).String()), Name: name,
			ModelID:       contract.ID(daptinid.InterfaceToDIR(row["llm_model_id"]).String()),
			ProviderID:    contract.ID(daptinid.InterfaceToDIR(row["llm_provider_id"]).String()),
			UpstreamModel: resource.StringOrEmpty(row["upstream_model"]), Operations: operations,
			Priority: int(priority), Weight: int(weight),
			RequestTimeout: time.Duration(requestTimeoutMS) * time.Millisecond,
			ConnectTimeout: time.Duration(connectTimeoutMS) * time.Millisecond,
			MaxConcurrency: maxConcurrency, RPM: rpm, TPM: tpm,
			Pricing: catalog.Pricing{
				InputMicrosPerMillion: pricing.Input, OutputMicrosPerMillion: pricing.Output,
				CacheReadMicrosPerMillion: pricing.CacheRead, CacheWriteMicrosPerMillion: pricing.CacheWrite,
				ReasoningMicrosPerMillion: pricing.Reasoning,
			},
			Parameters: []byte(resource.StringOrEmpty(row["parameters"])),
			HealthCheck: catalog.HealthCheck{
				Enabled: health.Enabled, Path: health.Path, Model: health.Model,
				Interval: time.Duration(health.IntervalMS) * time.Millisecond,
				Timeout:  time.Duration(health.TimeoutMS) * time.Millisecond, FailureThreshold: health.FailureThreshold,
			}, Enabled: true,
		})
	}

	sort.Slice(document.Providers, func(i, j int) bool { return document.Providers[i].ID < document.Providers[j].ID })
	sort.Slice(document.Models, func(i, j int) bool { return document.Models[i].ID < document.Models[j].ID })
	sort.Slice(document.Deployments, func(i, j int) bool { return document.Deployments[i].ID < document.Deployments[j].ID })
	type credentialVersion struct {
		Reference string `json:"reference"`
		Version   int64  `json:"version"`
	}
	credentialVersions := make([]credentialVersion, 0, len(credentialReferences))
	if len(credentialReferences) > 0 {
		storedReferences := make([]interface{}, 0, len(credentialReferences))
		for reference := range credentialReferences {
			referenceID := daptinid.InterfaceToDIR(reference)
			storedReferences = append(storedReferences, referenceID[:])
		}
		credentialRows, _, lookupErr := source.cruds["credential"].GetRowsByWhereClauseWithTransaction(
			"credential", nil, transaction, goqu.Ex{"reference_id": storedReferences},
		)
		if lookupErr != nil {
			return catalog.Document{}, fmt.Errorf("load LLM credential versions: %w", lookupErr)
		}
		if len(credentialRows) != len(credentialReferences) {
			return catalog.Document{}, errors.New("one or more LLM credential versions are unresolved")
		}
		for _, row := range credentialRows {
			reference := daptinid.InterfaceToDIR(row["reference_id"]).String()
			version, conversionErr := resource.ResourceRowInt64(row["version"])
			if conversionErr != nil {
				return catalog.Document{}, fmt.Errorf("decode LLM credential %s version: %w", reference, conversionErr)
			}
			credentialVersions = append(credentialVersions, credentialVersion{Reference: reference, Version: version})
		}
		sort.Slice(credentialVersions, func(i, j int) bool { return credentialVersions[i].Reference < credentialVersions[j].Reference })
	}
	payload, err := json.Marshal(struct {
		Catalog            catalog.Document    `json:"catalog"`
		CredentialVersions []credentialVersion `json:"credential_versions"`
	}{Catalog: document, CredentialVersions: credentialVersions})
	if err != nil {
		return catalog.Document{}, fmt.Errorf("fingerprint LLM catalog: %w", err)
	}
	fingerprint := sha256.Sum256(payload)

	if err := transaction.Commit(); err != nil {
		return catalog.Document{}, fmt.Errorf("commit LLM catalog read: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return catalog.Document{}, err
	}

	source.mu.Lock()
	defer source.mu.Unlock()
	if source.revision != 0 && bytes.Equal(source.fingerprint[:], fingerprint[:]) {
		return catalog.Document{}, catalog.ErrStaleRevision
	}
	if source.revision <= after {
		source.revision = after + 1
	} else {
		source.revision++
	}
	source.fingerprint = fingerprint
	document.Revision = source.revision
	document.GeneratedAt = time.Now().UTC()
	return document, nil
}

func decodeCatalogObject(raw string, destination interface{}) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("catalog JSON must contain exactly one document")
		}
		return fmt.Errorf("decode trailing catalog JSON: %w", err)
	}
	return nil
}
