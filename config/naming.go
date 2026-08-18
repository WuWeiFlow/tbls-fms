package config

import (
	"fmt"
	"strings"

	"github.com/gertd/go-pluralize"
)

var (
	pluralizeClient = pluralize.NewClient()
)

// Namer is a function type which is given a string and return a string.
type Namer func(string) string

// ContextNamer is a function type which is given a table and column name and returns a string.
type ContextNamer func(string, string) string

// NamingStrategy represents naming strategies.
type NamingStrategy struct {
	ParentTable            Namer
	ParentTableWithContext ContextNamer
	ParentColumn           Namer
	RequireParentPK        bool
	RequireSameType        bool
	AllowUniqueTableSuffix bool
}

// SelectNamingStrategy sets the naming strategy.
func SelectNamingStrategy(name string) (*NamingStrategy, error) {
	switch name {
	case "", "default":
		// default
		return &NamingStrategy{
			ParentTable:  defaultParentTableNamer,
			ParentColumn: defaultParentColumnNamer,
		}, nil

	case "singularTableName":
		return &NamingStrategy{
			ParentTable:  singularTableParentTableNamer,
			ParentColumn: singularTableParentColumnNamer,
		}, nil

	case "identical":
		return &NamingStrategy{
			ParentTable:  defaultParentTableNamer,
			ParentColumn: identicalParentColumnNamer,
		}, nil

	case "identicalSingularTableName":
		return &NamingStrategy{
			ParentTable:  singularTableParentTableNamer,
			ParentColumn: identicalParentColumnNamer,
		}, nil

	case "invertedSingularTableName":
		return &NamingStrategy{
			ParentTable:  invertedSingularTableParentTableNamer,
			ParentColumn: singularTableParentColumnNamer,
		}, nil

	case "fms":
		return &NamingStrategy{
			ParentTable:            emptyParentTableNamer,
			ParentTableWithContext: fmsParentTableNamer,
			ParentColumn:           fmsParentColumnNamer,
			RequireParentPK:        true,
			RequireSameType:        true,
			AllowUniqueTableSuffix: true,
		}, nil

	default:
		return nil, fmt.Errorf("naming strategy does not exist. strategy: %s", name)
	}
}

// ParentTableName alters the given name by Table.
func (ns *NamingStrategy) ParentTableName(name string) string {
	return ns.ParentTable(name)
}

// ParentTableNameFor alters the given column name by using its table as context when supported.
func (ns *NamingStrategy) ParentTableNameFor(tableName, columnName string) string {
	if ns.ParentTableWithContext != nil {
		return ns.ParentTableWithContext(tableName, columnName)
	}
	return ns.ParentTableName(columnName)
}

// ParentColumnName alters the given name by Column.
func (ns *NamingStrategy) ParentColumnName(name string) string {
	return ns.ParentColumn(name)
}

func defaultParentTableNamer(name string) string {
	index := strings.LastIndex(name, "_")

	if index == -1 || name[index+1:] != "id" {
		return ""
	}
	return pluralizeClient.Plural(name[:index])
}

func defaultParentColumnNamer(_ string) string {
	return "id"
}

func singularTableParentTableNamer(name string) string {
	index := strings.LastIndex(name, "_")

	if index == -1 || name[index+1:] != "id" {
		return ""
	}
	return pluralizeClient.Singular(name[:index])
}

func singularTableParentColumnNamer(_ string) string {
	return "id"
}

func identicalParentColumnNamer(name string) string {
	return name
}

func invertedSingularTableParentTableNamer(name string) string {
	index := strings.Index(name, "_")

	if index == -1 || name[:index] != "id" {
		return ""
	}
	return pluralizeClient.Singular(name[index+1:])
}

func emptyParentTableNamer(_ string) string {
	return ""
}

// fmsParentTableNamer maps a module-scoped child reference such as
// sr_order_hrs.order_id to the parent table sr_order.
func fmsParentTableNamer(tableName, columnName string) string {
	entityName := fmsEntityName(columnName)
	if entityName == "" {
		return ""
	}

	moduleEnd := strings.Index(tableName, "_")
	if moduleEnd <= 0 {
		return ""
	}

	return tableName[:moduleEnd] + "_" + entityName
}

func fmsEntityName(columnName string) string {
	if !strings.HasSuffix(columnName, "_id") {
		return ""
	}
	return strings.TrimSuffix(columnName, "_id")
}

func fmsParentColumnNamer(_ string) string {
	return "id_"
}
