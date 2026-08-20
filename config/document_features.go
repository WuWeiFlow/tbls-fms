package config

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	wildcard "github.com/IGLOU-EU/go-wildcard/v2"
	"github.com/k1LoW/tbls/schema"
	"gitlab.com/golang-commonmark/mdurl"
)

func renderAutoRelationDef(tmpl string, childTable *schema.Table, childColumn *schema.Column, parentTable *schema.Table, parentColumn *schema.Column) string {
	return strings.NewReplacer(
		"{childTable}", childTable.Name,
		"{childColumn}", childColumn.Name,
		"{parentTable}", parentTable.Name,
		"{parentColumn}", parentColumn.Name,
	).Replace(tmpl)
}

func renderAutoRelationCardinalities(tmpl string, cardinality, parentCardinality schema.Cardinality) string {
	return strings.NewReplacer(
		"{cardinality}", compactCardinality(cardinality),
		"{parentCardinality}", compactCardinality(parentCardinality),
	).Replace(tmpl)
}

func compactCardinality(cardinality schema.Cardinality) string {
	switch cardinality {
	case schema.ZeroOrOne:
		return "0..1"
	case schema.ExactlyOne:
		return "1"
	case schema.ZeroOrMore:
		return "0..N"
	case schema.OneOrMore:
		return "1..N"
	default:
		return "?"
	}
}

func (c *Config) viewpointConfigs(s *schema.Schema) ([]Viewpoint, error) {
	generated := []Viewpoint{}
	if c.ModuleViewpoints.Enabled {
		prefixes := map[string]struct{}{}
		for _, t := range s.Tables {
			prefixes[c.modulePrefix(t.Name)] = struct{}{}
		}
		ordered := make([]string, 0, len(prefixes))
		for prefix := range prefixes {
			if matchesModule(prefix, c.ModuleViewpoints.Include, c.ModuleViewpoints.Exclude) {
				ordered = append(ordered, prefix)
			}
		}
		sort.Strings(ordered)

		for _, prefix := range ordered {
			ownTables := map[string]struct{}{}
			tables := map[string]struct{}{}
			for _, t := range s.Tables {
				if c.modulePrefix(t.Name) == prefix {
					ownTables[t.Name] = struct{}{}
					tables[t.Name] = struct{}{}
				}
			}
			if c.ModuleViewpoints.CrossModule != "none" {
				for _, r := range s.Relations {
					if _, ok := ownTables[r.Table.Name]; ok {
						tables[r.ParentTable.Name] = struct{}{}
					}
					if c.ModuleViewpoints.CrossModule == "all" {
						if _, ok := ownTables[r.ParentTable.Name]; ok {
							tables[r.Table.Name] = struct{}{}
						}
					}
				}
			}
			tableNames := make([]string, 0, len(tables))
			for name := range tables {
				tableNames = append(tableNames, name)
			}
			sort.Strings(tableNames)
			generated = append(generated, Viewpoint{
				ID:     "module-" + safePathPart(prefix, "other"),
				Name:   strings.ToUpper(prefix) + " 模块",
				Desc:   fmt.Sprintf("按表名前缀 %s 自动生成的模块关系视图（跨模块模式：%s）", prefix, c.ModuleViewpoints.CrossModule),
				Tables: tableNames,
			})
		}
	}

	// A manual viewpoint with the same ID replaces its generated counterpart.
	manualIDs := map[string]struct{}{}
	for _, v := range c.Viewpoints {
		if v.ID != "" {
			manualIDs[v.ID] = struct{}{}
		}
	}
	viewpoints := make([]Viewpoint, 0, len(generated)+len(c.Viewpoints))
	for _, v := range generated {
		if _, overridden := manualIDs[v.ID]; !overridden {
			viewpoints = append(viewpoints, v)
		}
	}
	viewpoints = append(viewpoints, c.Viewpoints...)
	return viewpoints, nil
}

func matchesModule(prefix string, include, exclude []string) bool {
	included := len(include) == 0
	for _, pattern := range include {
		if wildcard.Match(pattern, prefix) {
			included = true
			break
		}
	}
	if !included {
		return false
	}
	for _, pattern := range exclude {
		if wildcard.Match(pattern, prefix) {
			return false
		}
	}
	return true
}

func (c *Config) tablePrefix(tableName string) string {
	return tablePrefix(tableName, c.TableDirectories.Separator, c.TableDirectories.Fallback)
}

func (c *Config) modulePrefix(tableName string) string {
	return tablePrefix(tableName, c.ModuleViewpoints.Separator, "other")
}

func tablePrefix(tableName, separator, fallback string) string {
	name := tableName
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	if separator != "" {
		if i := strings.Index(name, separator); i > 0 {
			return name[:i]
		}
	}
	return fallback
}

func safePathPart(value, fallback string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	result := strings.Trim(b.String(), ".-")
	if result == "" || result == "." || result == ".." {
		return fallback
	}
	return result
}

// TableRelativePath returns a slash-separated path relative to docPath.
func (c *Config) TableRelativePath(tableName, extension string) string {
	fileName := tableName + "." + extension
	if !c.TableDirectories.Enabled {
		return fileName
	}
	directory := safePathPart(c.tablePrefix(tableName), c.TableDirectories.Fallback)
	return path.Join(directory, fileName)
}

// TableCompactRelativePath returns the compact ER path for one table.
func (c *Config) TableCompactRelativePath(tableName, extension string) string {
	fileName := tableName + "-compact." + extension
	if !c.TableDirectories.Enabled {
		return fileName
	}
	directory := safePathPart(c.tablePrefix(tableName), c.TableDirectories.Fallback)
	return path.Join(directory, fileName)
}

// TableFilePath returns the filesystem path for one table artifact.
func (c *Config) TableFilePath(tableName, extension string) string {
	return filepath.Join(c.DocPath, filepath.FromSlash(c.TableRelativePath(tableName, extension)))
}

// TableCompactFilePath returns the filesystem path for one compact table ER.
func (c *Config) TableCompactFilePath(tableName, extension string) string {
	return filepath.Join(c.DocPath, filepath.FromSlash(c.TableCompactRelativePath(tableName, extension)))
}

// DocumentLink builds a Markdown link from a root or table document.
func (c *Config) DocumentLink(fromTable, targetRelativePath string) string {
	if c.BaseURL != "" {
		return c.BaseURL + encodeDocumentPath(targetRelativePath)
	}
	link := targetRelativePath
	if fromTable != "" {
		fromDir := filepath.Dir(filepath.FromSlash(c.TableRelativePath(fromTable, "md")))
		rel, err := filepath.Rel(fromDir, filepath.FromSlash(targetRelativePath))
		if err == nil {
			link = filepath.ToSlash(rel)
		}
	}
	return encodeDocumentPath(link)
}

func encodeDocumentPath(value string) string {
	parts := strings.Split(filepath.ToSlash(value), "/")
	for i, part := range parts {
		if part == ".." || part == "." {
			continue
		}
		parts[i] = mdurl.Encode(part)
	}
	return strings.Join(parts, "/")
}

// CompactSchema returns a clone showing every primary/relation column plus the
// first maxColumns ordinary columns of each table.
func CompactSchema(s *schema.Schema, maxColumns int) (*schema.Schema, error) {
	compact, err := s.CloneWithoutViewpoints()
	if err != nil {
		return nil, err
	}
	for _, r := range compact.Relations {
		r.HideForER = false
	}
	for tableIndex, t := range compact.Tables {
		ordinary := 0
		for columnIndex, column := range t.Columns {
			sourceColumn := s.Tables[tableIndex].Columns[columnIndex]
			related := len(column.ChildRelations) > 0 || len(column.ParentRelations) > 0
			if sourceColumn.PK || related {
				column.HideForER = false
				continue
			}
			column.HideForER = ordinary >= maxColumns
			ordinary++
		}
	}
	return compact, nil
}
