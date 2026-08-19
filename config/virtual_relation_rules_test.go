package config

import (
	"strings"
	"testing"

	"github.com/k1LoW/tbls/schema"
)

func TestVirtualRelationPriority(t *testing.T) {
	strategy, err := SelectNamingStrategy("fms")
	if err != nil {
		t.Fatal(err)
	}

	assetID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	staffID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	legacyStaffID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	projID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	assetRef := &schema.Column{Name: "asset_id", Type: "bigint"}
	createBy := &schema.Column{Name: "create_by", Type: "bigint"}
	projRef := &schema.Column{Name: "proj_id", Type: "bigint"}

	s := &schema.Schema{Tables: []*schema.Table{
		{Name: "eq_asset", Columns: []*schema.Column{assetID}},
		{Name: "pa_staff", Columns: []*schema.Column{staffID}},
		{Name: "legacy_staff", Columns: []*schema.Column{legacyStaffID}},
		{Name: "sys_proj", Columns: []*schema.Column{projID}},
		{Name: "sr_order", Columns: []*schema.Column{assetRef, createBy, projRef}},
	}}

	if err := mergeAdditionalRelations(s, []AdditionalRelation{{
		Table:         "sr_order",
		Columns:       []string{"create_by"},
		ParentTable:   "legacy_staff",
		ParentColumns: []string{"id_"},
		Def:           "Manual Relation",
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := mergeVirtualRelationRules(s, []VirtualRelationRule{
		{Columns: []string{"asset_id"}, ParentTable: "eq_asset", ParentColumn: "id_"},
		{Columns: []string{"create_by"}, ParentTable: "pa_staff", ParentColumn: "id_"},
	}); err != nil {
		t.Fatal(err)
	}
	mergeDetectedRelations(s, strategy)

	if got, want := len(s.Relations), 3; got != want {
		t.Fatalf("got %d relations, want %d", got, want)
	}
	assertRelation(t, s, "create_by", "legacy_staff", "Manual Relation")
	assertRelation(t, s, "asset_id", "eq_asset", "Mapped Relation")
	assertRelation(t, s, "proj_id", "sys_proj", "Detected Relation")
}

func TestMergeVirtualRelationRulesConflict(t *testing.T) {
	staffTeamID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	legacyTeamID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	staffTeamRef := &schema.Column{Name: "staff_team_id", Type: "bigint"}
	s := &schema.Schema{Tables: []*schema.Table{
		{Name: "org_staff_structure", Columns: []*schema.Column{staffTeamID}},
		{Name: "legacy_team", Columns: []*schema.Column{legacyTeamID}},
		{Name: "sr_order", Columns: []*schema.Column{staffTeamRef}},
	}}

	_, err := mergeVirtualRelationRules(s, []VirtualRelationRule{
		{Columns: []string{"staff_team_id"}, ParentTable: "org_staff_structure", ParentColumn: "id_"},
		{Columns: []string{"staff_team_id"}, ParentTable: "legacy_team", ParentColumn: "id_"},
	})
	if err == nil || !strings.Contains(err.Error(), "conflicting virtual relation rules for sr_order.staff_team_id") {
		t.Fatalf("got error %v, want mapping conflict", err)
	}
	if len(s.Relations) != 0 {
		t.Fatalf("conflicting rules must not partially add relations")
	}
}

func TestMergeVirtualRelationRulesPreservesManualPolymorphicRelations(t *testing.T) {
	orderID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	assetID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	ownerID := &schema.Column{Name: "owner_id", Type: "bigint"}
	s := &schema.Schema{Tables: []*schema.Table{
		{Name: "sr_order", Columns: []*schema.Column{orderID}},
		{Name: "eq_asset", Columns: []*schema.Column{assetID}},
		{Name: "biz_attachment", Columns: []*schema.Column{ownerID}},
	}}

	if err := mergeAdditionalRelations(s, []AdditionalRelation{
		{Table: "biz_attachment", Columns: []string{"owner_id"}, ParentTable: "sr_order", ParentColumns: []string{"id_"}, Def: "owner_type = ORDER"},
		{Table: "biz_attachment", Columns: []string{"owner_id"}, ParentTable: "eq_asset", ParentColumns: []string{"id_"}, Def: "owner_type = ASSET"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := mergeVirtualRelationRules(s, []VirtualRelationRule{{
		Columns: []string{"owner_id"}, ParentTable: "eq_asset", ParentColumn: "id_",
	}}); err != nil {
		t.Fatal(err)
	}

	if got, want := len(s.Relations), 2; got != want {
		t.Fatalf("got %d relations, want %d manual polymorphic relations", got, want)
	}
}

func TestMergeVirtualRelationRulesTableScope(t *testing.T) {
	assetID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	srAssetRef := &schema.Column{Name: "asset_id", Type: "bigint"}
	historyAssetRef := &schema.Column{Name: "asset_id", Type: "bigint"}
	eqAssetRef := &schema.Column{Name: "asset_id", Type: "bigint"}
	s := &schema.Schema{Tables: []*schema.Table{
		{Name: "eq_asset", Columns: []*schema.Column{assetID, eqAssetRef}},
		{Name: "sr_order", Columns: []*schema.Column{srAssetRef}},
		{Name: "sr_history_order", Columns: []*schema.Column{historyAssetRef}},
	}}

	if _, err := mergeVirtualRelationRules(s, []VirtualRelationRule{{
		Tables:        []string{"sr_*"},
		ExcludeTables: []string{"sr_history_*"},
		Columns:       []string{"asset_id"},
		ParentTable:   "eq_asset",
		ParentColumn:  "id_",
	}}); err != nil {
		t.Fatal(err)
	}

	if got, want := len(s.Relations), 1; got != want {
		t.Fatalf("got %d scoped relations, want %d", got, want)
	}
	if s.Relations[0].Table.Name != "sr_order" {
		t.Fatalf("unexpected scoped child table %s", s.Relations[0].Table.Name)
	}
}

func TestMergeVirtualRelationRulesSkipsTypeMismatch(t *testing.T) {
	staffID := &schema.Column{Name: "id_", Type: "bigint(20)", PK: true}
	invalidCreateBy := &schema.Column{Name: "create_by", Type: "varchar(255)"}
	validCreateBy := &schema.Column{Name: "create_by", Type: "bigint(20)"}
	s := &schema.Schema{Tables: []*schema.Table{
		{Name: "pa_staff", Columns: []*schema.Column{staffID}},
		{Name: "fm_user_form_data", Columns: []*schema.Column{invalidCreateBy}},
		{Name: "sr_order", Columns: []*schema.Column{validCreateBy}},
	}}

	warnings, err := mergeVirtualRelationRules(s, []VirtualRelationRule{{
		Columns: []string{"create_by"}, ParentTable: "pa_staff", ParentColumn: "id_",
	}})
	if err != nil {
		t.Fatalf("type mismatch must not stop generation: %v", err)
	}
	if got, want := len(warnings), 1; got != want {
		t.Fatalf("got %d warnings, want %d", got, want)
	}
	if !strings.Contains(warnings[0], "字段类型不匹配：fm_user_form_data.create_by（varchar(255)）与 pa_staff.id_（bigint(20)），已跳过该关系") {
		t.Fatalf("unexpected warning: %s", warnings[0])
	}
	if got, want := len(s.Relations), 1; got != want {
		t.Fatalf("got %d relations, want valid matches to continue", got)
	}
	assertRelation(t, s, "create_by", "pa_staff", "Mapped Relation")
}

func TestMergeDetectedRelationsFMSCrossModuleUniqueSuffix(t *testing.T) {
	strategy, err := SelectNamingStrategy("fms")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("unique candidate", func(t *testing.T) {
		assetID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
		assetRef := &schema.Column{Name: "asset_id", Type: "bigint"}
		s := &schema.Schema{Tables: []*schema.Table{
			{Name: "eq_asset", Columns: []*schema.Column{assetID}},
			{Name: "sr_order", Columns: []*schema.Column{assetRef}},
		}}

		mergeDetectedRelations(s, strategy)

		if got, want := len(s.Relations), 1; got != want {
			t.Fatalf("got %d relations, want %d", got, want)
		}
		assertRelation(t, s, "asset_id", "eq_asset", "Detected Relation")
	})

	t.Run("ambiguous candidates", func(t *testing.T) {
		eqAssetID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
		fmAssetID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
		assetRef := &schema.Column{Name: "asset_id", Type: "bigint"}
		s := &schema.Schema{Tables: []*schema.Table{
			{Name: "eq_asset", Columns: []*schema.Column{eqAssetID}},
			{Name: "fm_asset", Columns: []*schema.Column{fmAssetID}},
			{Name: "sr_order", Columns: []*schema.Column{assetRef}},
		}}

		mergeDetectedRelations(s, strategy)

		if len(s.Relations) != 0 {
			t.Fatalf("ambiguous cross-module candidates must not create a relation")
		}
	})
}

func assertRelation(t *testing.T, s *schema.Schema, childColumn, parentTable, def string) {
	t.Helper()
	for _, relation := range s.Relations {
		if len(relation.Columns) == 1 && relation.Columns[0].Name == childColumn {
			if relation.ParentTable.Name != parentTable || relation.Def != def {
				t.Fatalf("relation for %s is %s (%s), want %s (%s)", childColumn, relation.ParentTable.Name, relation.Def, parentTable, def)
			}
			return
		}
	}
	t.Fatalf("relation for child column %s not found", childColumn)
}
