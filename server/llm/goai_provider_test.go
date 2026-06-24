package llm

import (
	"context"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestCredentialBackedChatCompletionWithNilTransactionReturnsError(t *testing.T) {
	provider := credentialBackedTestProvider(t)

	_, err := provider.ChatCompletion(context.Background(), rootpojo.LLMProvider{
		Name:           "test-openai",
		ProviderType:   "openai",
		BaseUrl:        "http://127.0.0.1:1/v1",
		CredentialName: "llm-cred",
	}, OpenAIChatRequest{
		Model: "gpt-test",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "hello"},
		},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), "requires a database transaction") {
		t.Fatalf("expected missing transaction error, got %v", err)
	}
}

func TestCredentialBackedEmbeddingWithNilTransactionReturnsError(t *testing.T) {
	provider := credentialBackedTestProvider(t)

	_, err := provider.Embedding(context.Background(), rootpojo.LLMProvider{
		Name:           "test-openai",
		ProviderType:   "openai",
		BaseUrl:        "http://127.0.0.1:1/v1",
		CredentialName: "llm-cred",
	}, OpenAIEmbeddingRequest{
		Model: "embed-test",
		Input: "hello",
	}, nil)

	if err == nil || !strings.Contains(err.Error(), "requires a database transaction") {
		t.Fatalf("expected missing transaction error, got %v", err)
	}
}

func credentialBackedTestProvider(t *testing.T) *GoAIProvider {
	t.Helper()

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback()
	})

	if _, err := tx.Exec(`create table _config (
		id integer primary key,
		name text,
		configtype text,
		configstate text,
		configenv text,
		value text
	)`); err != nil {
		t.Fatalf("create _config: %v", err)
	}
	if _, err := tx.Exec(`create table credential (
		id integer primary key,
		name text not null,
		content text not null,
		user_account_id integer,
		reference_id blob,
		permission integer
	)`); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	secret := "0123456789abcdef0123456789abcdef"
	if _, err := tx.Exec(`insert into _config (name, configtype, configstate, configenv, value) values (?, ?, ?, ?, ?)`, "encryption.secret", "backend", "enabled", "", secret); err != nil {
		t.Fatalf("insert config: %v", err)
	}
	encryptedContent, err := resource.Encrypt([]byte(secret), `{"api_key":"test-key"}`)
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}
	if _, err := tx.Exec(`insert into credential (id, name, content, user_account_id, permission) values (?, ?, ?, ?, ?)`, 1, "llm-cred", encryptedContent, 1, 63); err != nil {
		t.Fatalf("insert credential: %v", err)
	}

	return NewGoAIProvider(map[string]*resource.DbResource{
		"credential": {
			ConfigStore: &resource.ConfigStore{},
		},
	})
}
