package gviz

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k1LoW/tbls/config"
	"github.com/k1LoW/tbls/schema"
)

func TestOutputWritesCompactAndPrefixERFiles(t *testing.T) {
	c, err := config.New()
	if err != nil {
		t.Fatal(err)
	}
	c.DocPath = t.TempDir()
	c.ER.Format = "svg"
	c.ER.Compact = config.CompactER{Enabled: true, MaxColumns: 0}
	c.TableDirectories = config.TableDirectories{Enabled: true, Separator: "_", Fallback: "other"}
	s := &schema.Schema{
		Name: "test",
		Tables: []*schema.Table{
			{Name: "sr_order", Type: "table", Columns: []*schema.Column{{Name: "id_", Type: "bigint", PK: true}, {Name: "remark", Type: "text"}}},
		},
	}

	if err := Output(s, c, true); err != nil {
		t.Fatal(err)
	}
	for _, relativePath := range []string{"schema.svg", "schema-compact.svg", "sr/sr_order.svg"} {
		info, err := os.Stat(filepath.Join(c.DocPath, filepath.FromSlash(relativePath)))
		if err != nil {
			t.Fatalf("missing %s: %v", relativePath, err)
		}
		if info.Size() == 0 {
			t.Fatalf("empty %s", relativePath)
		}
	}
}
