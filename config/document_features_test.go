package config

import (
	"os"
	"reflect"
	"testing"

	"github.com/k1LoW/tbls/schema"
)

func TestFMSExampleConfigDocumentsNewFeatures(t *testing.T) {
	buf, err := os.ReadFile("../deploy/tbls.example.yml")
	if err != nil {
		t.Fatal(err)
	}
	c := &Config{}
	if err := c.LoadConfig(buf); err != nil {
		t.Fatal(err)
	}
	if err := c.setDefault(); err != nil {
		t.Fatal(err)
	}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	if !c.ER.Compact.Enabled || c.ModuleViewpoints.CrossModule != "parents" || !c.TableDirectories.Enabled {
		t.Fatalf("new features are not enabled in example config: %#v", c)
	}
	if !c.ER.Comment {
		t.Fatal("ER comments are not enabled in example config")
	}
	if c.Lint.DuplicateRelations.Enabled {
		t.Fatal("duplicate relation lint should not be enabled by the example config")
	}
	if c.ER.RelationLabel.Distance != 3 || c.ER.RelationLabel.Angle != -90 || c.ER.NodeSep != 0.8 || c.ER.RankSep != 0.8 {
		t.Fatalf("unexpected ER relation label layout: %#v", c.ER)
	}
	if c.DetectVirtualRelations.AutoRelationDef != "关联 {parentTable} 表（自动匹配）" {
		t.Fatalf("unexpected auto relation definition: %q", c.DetectVirtualRelations.AutoRelationDef)
	}
}

func TestRenderAutoRelationDef(t *testing.T) {
	childTable := &schema.Table{Name: "sr_order"}
	childColumn := &schema.Column{Name: "asset_id"}
	parentTable := &schema.Table{Name: "eq_asset"}
	parentColumn := &schema.Column{Name: "id_"}
	got := renderAutoRelationDef("{childTable}.{childColumn} 关联 {parentTable}.{parentColumn}（自动匹配）", childTable, childColumn, parentTable, parentColumn)
	want := "sr_order.asset_id 关联 eq_asset.id_（自动匹配）"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestValidateERRelationLabelLayout(t *testing.T) {
	tests := []struct {
		name   string
		adjust func(*Config)
	}{
		{"negative distance", func(c *Config) { c.ER.RelationLabel.Distance = -1 }},
		{"angle too small", func(c *Config) { c.ER.RelationLabel.Angle = -181 }},
		{"angle too large", func(c *Config) { c.ER.RelationLabel.Angle = 181 }},
		{"negative node separation", func(c *Config) { c.ER.NodeSep = -1 }},
		{"negative rank separation", func(c *Config) { c.ER.RankSep = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New()
			if err != nil {
				t.Fatal(err)
			}
			tt.adjust(c)
			if err := c.validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestModuleViewpointsCrossModule(t *testing.T) {
	sr := &schema.Table{Name: "sr_order"}
	sys := &schema.Table{Name: "sys_biz_type"}
	crm := &schema.Table{Name: "crm_ticket"}
	s := &schema.Schema{
		Tables: []*schema.Table{sr, sys, crm},
		Relations: []*schema.Relation{
			{Table: sr, ParentTable: sys},
			{Table: crm, ParentTable: sr},
		},
	}

	for _, tt := range []struct {
		mode string
		want []string
	}{
		{mode: "none", want: []string{"sr_order"}},
		{mode: "parents", want: []string{"sr_order", "sys_biz_type"}},
		{mode: "all", want: []string{"crm_ticket", "sr_order", "sys_biz_type"}},
	} {
		c, err := New()
		if err != nil {
			t.Fatal(err)
		}
		c.ModuleViewpoints = ModuleViewpoints{Enabled: true, Include: []string{"sr"}, Separator: "_", CrossModule: tt.mode}
		viewpoints, err := c.viewpointConfigs(s)
		if err != nil {
			t.Fatal(err)
		}
		if len(viewpoints) != 1 {
			t.Fatalf("mode %s: got %d viewpoints", tt.mode, len(viewpoints))
		}
		if !reflect.DeepEqual(viewpoints[0].Tables, tt.want) {
			t.Fatalf("mode %s: got %v, want %v", tt.mode, viewpoints[0].Tables, tt.want)
		}
	}
}

func TestManualViewpointOverridesGeneratedModule(t *testing.T) {
	s := &schema.Schema{Tables: []*schema.Table{{Name: "sr_order"}}}
	c, err := New()
	if err != nil {
		t.Fatal(err)
	}
	c.ModuleViewpoints = ModuleViewpoints{Enabled: true, Include: []string{"sr"}, Separator: "_", CrossModule: "none"}
	c.Viewpoints = []Viewpoint{{ID: "module-sr", Name: "手工 SR", Desc: "手工配置", Tables: []string{"sr_order"}}}
	viewpoints, err := c.viewpointConfigs(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(viewpoints) != 1 || viewpoints[0].Name != "手工 SR" {
		t.Fatalf("manual viewpoint did not override generated viewpoint: %#v", viewpoints)
	}
}

func TestCompactSchema(t *testing.T) {
	parentID := &schema.Column{Name: "id_", PK: true}
	parent := &schema.Table{Name: "eq_asset", Columns: []*schema.Column{parentID, {Name: "name"}}}
	childID := &schema.Column{Name: "id_", PK: true}
	first := &schema.Column{Name: "order_no"}
	related := &schema.Column{Name: "asset_id"}
	extra := &schema.Column{Name: "remark"}
	child := &schema.Table{Name: "sr_order", Columns: []*schema.Column{childID, first, related, extra}}
	relation := &schema.Relation{Table: child, Columns: []*schema.Column{related}, ParentTable: parent, ParentColumns: []*schema.Column{parentID}}
	related.ParentRelations = []*schema.Relation{relation}
	parentID.ChildRelations = []*schema.Relation{relation}
	s := &schema.Schema{Tables: []*schema.Table{child, parent}, Relations: []*schema.Relation{relation}}

	compact, err := CompactSchema(s, 1)
	if err != nil {
		t.Fatal(err)
	}
	got := compact.Tables[0].Columns
	if got[0].HideForER || got[1].HideForER || got[2].HideForER || !got[3].HideForER {
		t.Fatalf("unexpected compact visibility: %v %v %v %v", got[0].HideForER, got[1].HideForER, got[2].HideForER, got[3].HideForER)
	}
	if extra.HideForER {
		t.Fatal("compact schema must not mutate source schema")
	}
}

func TestTablePathsAndLinks(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatal(err)
	}
	c.TableDirectories = TableDirectories{Enabled: true, Separator: "_", Fallback: "other"}
	if got, want := c.TableRelativePath("sr_order", "md"), "sr/sr_order.md"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := c.TableCompactRelativePath("sr_order", "svg"), "sr/sr_order-compact.svg"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := c.DocumentLink("sr_order", c.TableRelativePath("eq_asset", "md")), "../eq/eq_asset.md"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := c.DocumentLink("sr_order", "viewpoint-module-sr.md"), "../viewpoint-module-sr.md"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
