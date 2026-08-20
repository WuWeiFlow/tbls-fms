package xlsx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/k1LoW/tbls/config"
	"github.com/k1LoW/tbls/schema"
)

func TestRelationDescription(t *testing.T) {
	s, childColumn, parentColumn := searchableWorkbookSchema(t)

	if got, want := relationDescription(childColumn), "关联sr_order表id_字段"; got != want {
		t.Fatalf("child relation description = %q, want %q", got, want)
	}
	if got, want := relationDescription(parentColumn), "被sr_order_hrs表order_id字段关联"; got != want {
		t.Fatalf("parent relation description = %q, want %q", got, want)
	}
	if len(s.Relations) != 1 {
		t.Fatalf("relations = %d, want 1", len(s.Relations))
	}
}

func TestOutputSchemaCreatesSearchableWorkbook(t *testing.T) {
	s, _, _ := searchableWorkbookSchema(t)
	c, err := config.New()
	if err != nil {
		t.Fatal(err)
	}
	c.ModuleViewpoints = config.ModuleViewpoints{Enabled: true, Separator: "_"}

	var output bytes.Buffer
	if err := New(c).OutputSchema(&output, s); err != nil {
		t.Fatal(err)
	}
	files := unzipWorkbook(t, output.Bytes())

	workbook := files["xl/workbook.xml"]
	for _, name := range []string{moduleSheetName, tableSheetName, columnSheetName, relationSheetName, constraintSheetName} {
		if !strings.Contains(workbook, `name="`+name+`"`) {
			t.Errorf("workbook does not contain sheet %q", name)
		}
	}
	if got, want := strings.Count(workbook, "<sheet "), 5; got != want {
		t.Fatalf("sheet count = %d, want %d", got, want)
	}
	if strings.Contains(workbook, `name="sr_order"`) || strings.Contains(workbook, `name="sr_order_hrs"`) {
		t.Fatal("workbook must not create one sheet per database table")
	}

	sharedStrings := files["xl/sharedStrings.xml"]
	for _, value := range []string{
		"模块名称",
		"关联字段",
		"关系描述",
		"关联sr_order表id_字段",
		"被sr_order_hrs表order_id字段关联",
		"数据库外键",
		"FOREIGN KEY (order_id) REFERENCES sr_order(id_)",
	} {
		if !strings.Contains(sharedStrings, value) {
			t.Errorf("shared strings do not contain %q", value)
		}
	}

	worksheets := ""
	for name, content := range files {
		if strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml") {
			worksheets += content
		}
	}
	if strings.Count(worksheets, "<hyperlink ") < 7 {
		t.Fatalf("expected module, table, and relation navigation links")
	}
	if got, want := strings.Count(worksheets, "<autoFilter "), 5; got != want {
		t.Fatalf("auto filter count = %d, want %d", got, want)
	}
	if got, want := strings.Count(worksheets, `state="frozen"`), 5; got != want {
		t.Fatalf("frozen header count = %d, want %d", got, want)
	}
	for _, target := range []string{tableSheetName, columnSheetName} {
		if !strings.Contains(worksheets, target) {
			t.Errorf("navigation formulas do not reference %q", target)
		}
	}
}

func TestOutputSchemaUsesTopLevelRelationsForColumnDescriptions(t *testing.T) {
	orderID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	hoursOrderID := &schema.Column{Name: "order_id", Type: "bigint"}
	s := &schema.Schema{
		Name: "fms",
		Tables: []*schema.Table{
			{Name: "sr_order", Type: "table", Columns: []*schema.Column{orderID}},
			{Name: "sr_order_hrs", Type: "table", Columns: []*schema.Column{hoursOrderID}},
		},
		Relations: []*schema.Relation{{
			Table:         &schema.Table{Name: "sr_order_hrs"},
			Columns:       []*schema.Column{{Name: "order_id"}},
			ParentTable:   &schema.Table{Name: "sr_order"},
			ParentColumns: []*schema.Column{{Name: "id_"}},
		}},
	}
	if len(hoursOrderID.ParentRelations) != 0 || len(orderID.ChildRelations) != 0 {
		t.Fatal("test requires relations to exist only in Schema.Relations")
	}

	c, err := config.New()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := New(c).OutputSchema(&output, s); err != nil {
		t.Fatal(err)
	}

	sharedStrings := unzipWorkbook(t, output.Bytes())["xl/sharedStrings.xml"]
	for _, value := range []string{
		"关联sr_order表id_字段",
		"被sr_order_hrs表order_id字段关联",
	} {
		if !strings.Contains(sharedStrings, value) {
			t.Errorf("shared strings do not contain %q", value)
		}
	}
}

func searchableWorkbookSchema(t *testing.T) (*schema.Schema, *schema.Column, *schema.Column) {
	t.Helper()
	assetID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	orderID := &schema.Column{Name: "id_", Type: "bigint", PK: true, Comment: "工单主键"}
	hoursID := &schema.Column{Name: "id_", Type: "bigint", PK: true}
	hoursOrderID := &schema.Column{Name: "order_id", Type: "bigint", FK: true, Comment: "所属工单"}
	asset := &schema.Table{Name: "eq_asset", Type: "table", Comment: "设备", Columns: []*schema.Column{assetID}}
	order := &schema.Table{
		Name:        "sr_order",
		Type:        "table",
		Comment:     "工单",
		Columns:     []*schema.Column{orderID},
		Constraints: []*schema.Constraint{{Name: "pk_sr_order", Type: "PRIMARY KEY", Def: "PRIMARY KEY (id_)"}},
	}
	hours := &schema.Table{Name: "sr_order_hrs", Type: "table", Comment: "工时", Columns: []*schema.Column{hoursID, hoursOrderID}}
	relation := &schema.Relation{
		Table:             hours,
		Columns:           []*schema.Column{hoursOrderID},
		ParentTable:       order,
		ParentColumns:     []*schema.Column{orderID},
		Cardinality:       schema.ZeroOrMore,
		ParentCardinality: schema.ExactlyOne,
		Def:               "FOREIGN KEY (order_id) REFERENCES sr_order(id_)",
	}
	s := &schema.Schema{Name: "fms", Tables: []*schema.Table{hours, order, asset}, Relations: []*schema.Relation{relation}}
	if err := s.Repair(); err != nil {
		t.Fatal(err)
	}
	return s, hoursOrderID, orderID
}

func unzipWorkbook(t *testing.T, data []byte) map[string]string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, file := range reader.File {
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = string(content)
	}
	return files
}
