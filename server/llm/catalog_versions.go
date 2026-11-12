package llm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

type catalogRecordVersion struct {
	Reference string
	Version   int64
}

type catalogVersions struct {
	Providers   []catalogRecordVersion
	Models      []catalogRecordVersion
	Deployments []catalogRecordVersion
	Credentials []catalogRecordVersion
}

func (versions catalogVersions) fingerprint() ([sha256.Size]byte, error) {
	for _, records := range [][]catalogRecordVersion{versions.Providers, versions.Models, versions.Deployments, versions.Credentials} {
		sort.Slice(records, func(i, j int) bool { return records[i].Reference < records[j].Reference })
	}
	payload, err := json.Marshal(versions)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(payload), nil
}

func (source *daptinCatalog) readVersions(ctx context.Context, tx *sqlx.Tx) (catalogVersions, error) {
	var versions catalogVersions
	for _, table := range []struct {
		name    string
		records *[]catalogRecordVersion
	}{
		{"llm_provider", &versions.Providers}, {"llm_model", &versions.Models}, {"llm_deployment", &versions.Deployments},
	} {
		records, err := readCatalogRecordVersions(ctx, tx, table.name, goqu.Ex{"enable": true})
		if err != nil {
			return versions, err
		}
		*table.records = records
	}
	var err error
	versions.Credentials, err = readCatalogCredentialVersions(ctx, tx, source.credentialReferences)
	return versions, err
}

func readCatalogCredentialVersions(ctx context.Context, tx *sqlx.Tx, references []string) ([]catalogRecordVersion, error) {
	if len(references) == 0 {
		return nil, nil
	}
	const batchSize = 256
	var result []catalogRecordVersion
	for start := 0; start < len(references); start += batchSize {
		batch := references[start:min(start+batchSize, len(references))]
		stored := make([]interface{}, 0, len(batch))
		for _, reference := range batch {
			id := daptinid.InterfaceToDIR(reference)
			stored = append(stored, id[:])
		}
		versions, err := readCatalogRecordVersions(ctx, tx, "credential", goqu.Ex{"reference_id": stored})
		if err != nil {
			return nil, err
		}
		result = append(result, versions...)
	}
	return result, nil
}

// Project only identity and version. Keyset pagination bounds each read without
// truncating the configuration or fetching credential contents and relations.
func readCatalogRecordVersions(ctx context.Context, tx *sqlx.Tx, table string, filters ...goqu.Ex) ([]catalogRecordVersion, error) {
	const pageSize = 256
	var result []catalogRecordVersion
	var cursor daptinid.DaptinReferenceId
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		queries := append([]goqu.Ex(nil), filters...)
		if cursor != daptinid.NullReferenceId {
			queries = append(queries, goqu.Ex{"reference_id": goqu.Op{"gt": cursor[:]}})
		}
		rows, err := resource.GetLimitedOrderedRowsWithTransaction(table, []string{"reference_id", "version"}, "reference_id", tx, pageSize, queries...)
		if err != nil {
			return nil, fmt.Errorf("read %s catalog versions: %w", table, err)
		}
		records, err := catalogVersionsFromRows(table, rows)
		if err != nil {
			return nil, err
		}
		result = append(result, records...)
		if len(rows) < pageSize {
			return result, nil
		}
		cursor = daptinid.InterfaceToDIR(records[len(records)-1].Reference)
	}
}

func catalogVersionsFromRows(table string, rows []map[string]interface{}) ([]catalogRecordVersion, error) {
	var records []catalogRecordVersion
	for _, row := range rows {
		reference := daptinid.InterfaceToDIR(row["reference_id"])
		version, err := resource.ResourceRowInt64(row["version"])
		if reference == daptinid.NullReferenceId || err != nil || version < 1 {
			return nil, fmt.Errorf("invalid %s catalog record version", table)
		}
		records = append(records, catalogRecordVersion{Reference: reference.String(), Version: version})
	}
	return records, nil
}
