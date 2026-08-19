package md

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k1LoW/tbls/config"
	"github.com/k1LoW/tbls/schema"
)

func TestOutputGroupsTableDocumentsByPrefix(t *testing.T) {
	c, err := config.New()
	if err != nil {
		t.Fatal(err)
	}
	c.DocPath = t.TempDir()
	c.ER.Skip = true
	c.TableDirectories = config.TableDirectories{Enabled: true, Separator: "_", Fallback: "other"}
	s := &schema.Schema{
		Name: "test",
		Tables: []*schema.Table{
			{Name: "sr_order", Type: "table", Columns: []*schema.Column{{Name: "id_", Type: "bigint", PK: true}}},
			{Name: "eq_asset", Type: "table", Columns: []*schema.Column{{Name: "id_", Type: "bigint", PK: true}}},
		},
	}

	if err := Output(s, c, true); err != nil {
		t.Fatal(err)
	}
	for _, relativePath := range []string{"README.md", "sr/sr_order.md", "eq/eq_asset.md"} {
		if _, err := os.Stat(filepath.Join(c.DocPath, filepath.FromSlash(relativePath))); err != nil {
			t.Fatalf("missing %s: %v", relativePath, err)
		}
	}
	if _, err := os.Stat(filepath.Join(c.DocPath, "sr_order.md")); !os.IsNotExist(err) {
		t.Fatalf("root table document should not exist: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(c.DocPath, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "(sr/sr_order.md)") || !strings.Contains(string(readme), "(eq/eq_asset.md)") {
		t.Fatalf("README does not contain prefix-directory links:\n%s", readme)
	}
	diff, err := DiffSchemaAndDocs(c.DocPath, s, c)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Fatalf("freshly generated prefix-directory docs should have no diff:\n%s", diff)
	}
}

func TestDiffSchemaAndDocsDoesNotScanSubdirectoriesByDefault(t *testing.T) {
	c, err := config.New()
	if err != nil {
		t.Fatal(err)
	}
	c.DocPath = t.TempDir()
	c.ER.Skip = true
	s := &schema.Schema{
		Name: "test",
		Tables: []*schema.Table{
			{Name: "sr_order", Type: "table", Columns: []*schema.Column{{Name: "id_", Type: "bigint", PK: true}}},
		},
	}

	if err := Output(s, c, true); err != nil {
		t.Fatal(err)
	}
	unrelatedDir := filepath.Join(c.DocPath, "unrelated")
	if err := os.MkdirAll(unrelatedDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unrelatedDir, "notes.md"), []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}

	diff, err := DiffSchemaAndDocs(c.DocPath, s, c)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Fatalf("nested Markdown should be ignored when tableDirectories is disabled:\n%s", diff)
	}
}
