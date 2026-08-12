package config

import (
	"testing"

	"github.com/k1LoW/tbls/schema"
)

func TestSingularTableParentTableNamer(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"user_id", "user"},
	}

	for _, tt := range tests {
		got := singularTableParentTableNamer(tt.name)

		if got != tt.want {
			t.Errorf("name %v\ngot %v\nwant %v", tt.name, got, tt.want)
		}
	}
}

func TestSingularTableParentColumnNamer(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		// normal
		{"user_id", "id"},
	}

	for _, tt := range tests {
		got := singularTableParentColumnNamer(tt.name)

		if got != tt.want {
			t.Errorf("name %v\ngot %v\nwant %v", tt.name, got, tt.want)
		}
	}
}

func TestIdenticalParentColumnNamer(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		// normal
		{"user_id", "user_id"},
	}

	for _, tt := range tests {
		got := identicalParentColumnNamer(tt.name)

		if got != tt.want {
			t.Errorf("name %v\ngot %v\nwant %v", tt.name, got, tt.want)
		}
	}
}

func TestInvertedSingularTableParentTableNamer(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"id_user", "user"},
		{"id_category", "category"},
		{"id_users", "user"},
		{"user_id", ""},
		{"id", ""},
		{"userid", ""},
	}

	for _, tt := range tests {
		got := invertedSingularTableParentTableNamer(tt.name)

		if got != tt.want {
			t.Errorf("name %v\ngot %v\nwant %v", tt.name, got, tt.want)
		}
	}
}

func TestSelectNamingStrategy_InvertedSingularTableName(t *testing.T) {
	strategy, err := SelectNamingStrategy("invertedSingularTableName")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		columnName      string
		wantParentTable string
		wantParentCol   string
	}{
		{"id_user", "user", "id"},
		{"id_category", "category", "id"},
	}

	for _, tt := range tests {
		gotTable := strategy.ParentTableName(tt.columnName)
		gotCol := strategy.ParentColumnName(tt.columnName)

		if gotTable != tt.wantParentTable {
			t.Errorf("ParentTableName(%v)\ngot %v\nwant %v", tt.columnName, gotTable, tt.wantParentTable)
		}
		if gotCol != tt.wantParentCol {
			t.Errorf("ParentColumnName(%v)\ngot %v\nwant %v", tt.columnName, gotCol, tt.wantParentCol)
		}
	}
}

func TestSelectNamingStrategy_FMS(t *testing.T) {
	strategy, err := SelectNamingStrategy("fms")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		tableName       string
		columnName      string
		wantParentTable string
		wantParentCol   string
	}{
		{"sr_order_hrs", "order_id", "sr_order", "id_"},
		{"sr_order_item", "order_type_id", "sr_order_type", "id_"},
		{"sys_user_role", "user_id", "sys_user", "id_"},
		{"sr_order_hrs", "created_by", "", "id_"},
	}

	for _, tt := range tests {
		gotTable := strategy.ParentTableNameFor(tt.tableName, tt.columnName)
		gotCol := strategy.ParentColumnName(tt.columnName)

		if gotTable != tt.wantParentTable {
			t.Errorf("ParentTableNameFor(%v, %v)\ngot %v\nwant %v", tt.tableName, tt.columnName, gotTable, tt.wantParentTable)
		}
		if gotCol != tt.wantParentCol {
			t.Errorf("ParentColumnName(%v)\ngot %v\nwant %v", tt.columnName, gotCol, tt.wantParentCol)
		}
	}

	if !strategy.RequireParentPK {
		t.Error("fms strategy must require the inferred parent column to be a primary key")
	}
	if !strategy.RequireSameType {
		t.Error("fms strategy must require matching child and parent column types")
	}
}

func TestMergeDetectedRelations_FMS(t *testing.T) {
	strategy, err := SelectNamingStrategy("fms")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	orderID := &schema.Column{Name: "id_", Type: "BIGINT", PK: true}
	orderRef := &schema.Column{Name: "order_id", Type: "bigint"}
	userIDWithoutPK := &schema.Column{Name: "id_", Type: "bigint"}
	userRef := &schema.Column{Name: "user_id", Type: "bigint"}
	customerID := &schema.Column{Name: "id_", Type: "varchar(36)", PK: true}
	customerRef := &schema.Column{Name: "customer_id", Type: "bigint"}

	s := &schema.Schema{Tables: []*schema.Table{
		{Name: "sr_order", Columns: []*schema.Column{orderID}},
		{Name: "sr_order_hrs", Columns: []*schema.Column{orderRef, userRef, customerRef}},
		{Name: "sr_user", Columns: []*schema.Column{userIDWithoutPK}},
		{Name: "sr_customer", Columns: []*schema.Column{customerID}},
	}}

	mergeDetectedRelations(s, strategy)

	if len(s.Relations) != 1 {
		t.Fatalf("got %d detected relations, want 1", len(s.Relations))
	}
	relation := s.Relations[0]
	if relation.Table.Name != "sr_order_hrs" || relation.Columns[0].Name != "order_id" {
		t.Errorf("unexpected child relation: %s.%s", relation.Table.Name, relation.Columns[0].Name)
	}
	if relation.ParentTable.Name != "sr_order" || relation.ParentColumns[0].Name != "id_" {
		t.Errorf("unexpected parent relation: %s.%s", relation.ParentTable.Name, relation.ParentColumns[0].Name)
	}
}
