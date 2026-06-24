package resource

import "testing"

func TestOutboxBelongsToMailServer(t *testing.T) {
	for _, relation := range StandardRelations {
		if relation.GetSubject() == "outbox" &&
			relation.GetRelation() == "belongs_to" &&
			relation.GetObject() == "mail_server" {
			return
		}
	}
	t.Fatal("expected StandardRelations to include outbox belongs_to mail_server")
}

func TestMailServerHostnameIsUnique(t *testing.T) {
	for _, table := range StandardTables {
		if table.TableName != "mail_server" {
			continue
		}
		column, ok := table.GetColumnByName("hostname")
		if !ok {
			t.Fatal("mail_server.hostname column not found")
		}
		if !column.IsUnique {
			t.Fatal("mail_server.hostname must be unique because mail.send resolves mail_server by hostname")
		}
		return
	}
	t.Fatal("mail_server table not found")
}
