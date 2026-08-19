package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	wildcard "github.com/IGLOU-EU/go-wildcard/v2"
	"github.com/aquasecurity/go-version/pkg/version"
	"github.com/goccy/go-yaml"
	"github.com/k1LoW/errors"
	"github.com/k1LoW/expand"
	"github.com/k1LoW/tbls/dict"
	"github.com/k1LoW/tbls/schema"
	ver "github.com/k1LoW/tbls/version"
	"github.com/samber/lo"
)

const DefaultDocPath = "dbdoc"

var DefaultConfigFilePaths = []string{".tbls.yml", "tbls.yml", ".tbls.yaml", "tbls.yaml"}

// DefaultERFormat is the default ER diagram format.
const DefaultERFormat = "svg"

var SupportERFormat = []string{"png", "jpg", "svg", "mermaid"}

const SchemaFileName = "schema.json"

const VirtualRelationWarningsFileName = "virtual-relation-warnings.log"

// DefaultERDistance is the default distance between tables that display relations in the ER.
var DefaultERDistance = 1

// Config is tbls config.
type Config struct {
	Name   string   `yaml:"name"`
	Desc   string   `yaml:"desc,omitempty"`
	Labels []string `yaml:"labels,omitempty"`
	DSN    DSN      `yaml:"dsn"`
	// Directory of schema document
	DocPath                string                 `yaml:"docPath"`
	Format                 Format                 `yaml:"format,omitempty"`
	ER                     ER                     `yaml:"er,omitempty"`
	Include                []string               `yaml:"include,omitempty"`
	Exclude                []string               `yaml:"exclude,omitempty"`
	Distance               int                    `yaml:"distance,omitempty"`
	Lint                   Lint                   `yaml:"lint,omitempty"`
	LintExclude            []string               `yaml:"lintExclude,omitempty"`
	Viewpoints             []Viewpoint            `yaml:"viewpoints,omitempty"`
	Relations              []AdditionalRelation   `yaml:"relations,omitempty"`
	Comments               []AdditionalComment    `yaml:"comments,omitempty"`
	Dict                   dict.Dict              `yaml:"dict,omitempty"`
	Templates              Templates              `yaml:"templates,omitempty"`
	DetectVirtualRelations DetectVirtualRelations `yaml:"detectVirtualRelations,omitempty"`
	BaseURL                string                 `yaml:"baseUrl,omitempty"`
	RequiredVersion        string                 `yaml:"requiredVersion,omitempty"`
	DisableOutputSchema    bool                   `yaml:"disableOutputSchema,omitempty"`
	MergedDict             dict.Dict              `yaml:"-"`

	// Table labels to be included
	includeLabels []string

	// Path of config file
	Path string `yaml:"-"`
	root string `yaml:"-"`
}

type DSN struct {
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

// Format is document format setting.
type Format struct {
	Adjust                   bool     `yaml:"adjust,omitempty"`
	Sort                     bool     `yaml:"sort,omitempty"`
	Number                   bool     `yaml:"number,omitempty"`
	ShowOnlyFirstParagraph   bool     `yaml:"showOnlyFirstParagraph,omitempty"`
	HideColumnsWithoutValues []string `yaml:"hideColumnsWithoutValues,omitempty"`
}

// ER is er setting.
type ER struct {
	Skip            bool             `yaml:"skip,omitempty"`
	Format          string           `yaml:"format,omitempty"`
	Comment         bool             `yaml:"comment,omitempty"`
	HideDef         bool             `yaml:"hideDef,omitempty"`
	ShowColumnTypes *ShowColumnTypes `yaml:"showColumnTypes,omitempty"`
	Distance        *int             `yaml:"distance,omitempty"`
	Font            string           `yaml:"font,omitempty"`
}

// ShowColumnTypes is show column setting for ER diagram.
type ShowColumnTypes struct {
	Related bool `yaml:"related,omitempty"`
	Primary bool `yaml:"primary,omitempty"`
}

// AdditionalRelation is the struct for table relation from yaml.
type AdditionalRelation struct {
	Table             string   `yaml:"table"`
	Columns           []string `yaml:"columns"`
	Cardinality       string   `yaml:"cardinality,omitempty"`
	ParentTable       string   `yaml:"parentTable"`
	ParentColumns     []string `yaml:"parentColumns"`
	ParentCardinality string   `yaml:"parentCardinality,omitempty"`
	Def               string   `yaml:"def,omitempty"`
	Override          bool     `yaml:"override,omitempty"`
}

// AdditionalComment is the struct for table relation from yaml.
type AdditionalComment struct {
	Table              string              `yaml:"table"`
	TableComment       string              `yaml:"tableComment,omitempty"`
	ColumnComments     map[string]string   `yaml:"columnComments,omitempty"`
	ColumnLabels       map[string][]string `yaml:"columnLabels,omitempty"`
	IndexComments      map[string]string   `yaml:"indexComments,omitempty"`
	ConstraintComments map[string]string   `yaml:"constraintComments,omitempty"`
	TriggerComments    map[string]string   `yaml:"triggerComments,omitempty"`
	Labels             []string            `yaml:"labels,omitempty"`
}

type DetectVirtualRelations struct {
	Enabled  bool                  `yaml:"enabled,omitempty"`
	Strategy string                `yaml:"strategy,omitempty"`
	Rules    []VirtualRelationRule `yaml:"rules,omitempty"`
}

// VirtualRelationRule maps recurring child columns to one parent key.
type VirtualRelationRule struct {
	Tables        []string `yaml:"tables,omitempty"`
	ExcludeTables []string `yaml:"excludeTables,omitempty"`
	Columns       []string `yaml:"columns"`
	ParentTable   string   `yaml:"parentTable"`
	ParentColumn  string   `yaml:"parentColumn"`
	Def           string   `yaml:"def,omitempty"`
}

// Option function change Config.
type Option func(*Config) error

// DSNURL return Option set Config.DSN.URL.
func DSNURL(dsn string) Option {
	return func(c *Config) error {
		c.DSN.URL = dsn
		return nil
	}
}

// DocPath return Option set Config.DocPath.
func DocPath(docPath string) Option {
	return func(c *Config) error {
		c.DocPath = docPath
		return nil
	}
}

// Adjust return Option set Config.Format.Adjust.
func Adjust(adjust bool) Option {
	return func(c *Config) error {
		if adjust {
			c.Format.Adjust = adjust
		}
		return nil
	}
}

// Sort return Option set Config.Format.Sort.
func Sort(sort bool) Option {
	return func(c *Config) error {
		if sort {
			c.Format.Sort = sort
		}
		return nil
	}
}

// ERSkip return Option set Config.ER.Skip.
func ERSkip(skip bool) Option {
	return func(c *Config) error {
		c.ER.Skip = skip
		return nil
	}
}

// ERFormat return Option set Config.ER.Format.
func ERFormat(erFormat string) Option {
	return func(c *Config) error {
		if erFormat != "" {
			c.ER.Format = erFormat
		}
		return nil
	}
}

// Distance return Option set Config.Distance.
func Distance(distance int) Option {
	return func(c *Config) error {
		c.Distance = distance
		return nil
	}
}

// BaseURL return Option set Config.BaseURL.
func BaseURL(baseURL string) Option {
	return func(c *Config) error {
		if baseURL != "" {
			c.BaseURL = baseURL
		}
		return nil
	}
}

// Include return Option set Config.Include.
func Include(i []string) Option {
	return func(c *Config) error {
		if len(i) > 0 {
			c.Include = i
		}
		return nil
	}
}

// Exclude return Option set Config.Exclude.
func Exclude(e []string) Option {
	return func(c *Config) error {
		if len(e) > 0 {
			c.Exclude = e
		}
		return nil
	}
}

// IncludeLabels return Option set Config.includeLabels.
func IncludeLabels(l []string) Option {
	return func(c *Config) error {
		if len(l) > 0 {
			c.includeLabels = l
		}
		return nil
	}
}

// New return Config.
func New() (*Config, error) {
	c := Config{}
	err := c.setDefault()
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Load config with all method.
func (c *Config) Load(configPath string, options ...Option) error {
	if err := c.LoadConfigFile(configPath); err != nil {
		return err
	}

	if err := c.LoadEnviron(); err != nil {
		return err
	}

	if err := c.LoadOption(options...); err != nil {
		return err
	}

	if err := c.setDefault(); err != nil {
		return err
	}

	if err := c.validate(); err != nil {
		return err
	}

	return nil
}

// LoadOptions load options.
func (c *Config) LoadOption(options ...Option) error {
	for _, option := range options {
		if err := option(c); err != nil {
			return err
		}
	}
	return nil
}

// set default setting.
func (c *Config) setDefault() error {
	if c.DocPath == "" {
		c.DocPath = DefaultDocPath
	}

	if c.ER.Format == "" {
		c.ER.Format = DefaultERFormat
	}

	if c.ER.Distance == nil {
		c.ER.Distance = &DefaultERDistance
	}

	return nil
}

func (c *Config) checkVersion(sv string) error {
	if sv == "dev" {
		return nil
	}
	if c.RequiredVersion == "" {
		return nil
	}
	cons, err := version.NewConstraints(c.RequiredVersion)
	if err != nil {
		return err
	}
	v, err := version.Parse(sv)
	if err != nil {
		return err
	}
	if !cons.Check(v) {
		return fmt.Errorf("the required tbls version for the configuration is '%s'. however, the running tbls version is '%s'", c.RequiredVersion, sv)
	}

	return nil
}

func (c *Config) validate() error {
	if err := c.checkVersion(ver.Version); err != nil {
		return err
	}
	if !lo.Contains(SupportERFormat, c.ER.Format) {
		return fmt.Errorf("unsupported ER format: %s", c.ER.Format)
	}
	seenViewpointIDs := map[string]int{}
	seenViewpointNames := map[string]int{}
	for i, v := range c.Viewpoints {
		if v.Name == "" {
			return fmt.Errorf("viewpoints[%d] name is required", i)
		}
		if v.Desc == "" {
			return fmt.Errorf("viewpoints[%d] description is required", i)
		}
		if v.ID != "" {
			// id is embedded into output file paths (e.g. viewpoint-<id>.md), so path
			// separators would allow writing outside the intended output directory.
			if strings.ContainsAny(v.ID, `/\`) {
				return fmt.Errorf("viewpoints[%d] id '%s' must not contain path separators ('/' or '\\')", i, v.ID)
			}
			if j, ok := seenViewpointIDs[v.ID]; ok {
				return fmt.Errorf("viewpoints[%d] id '%s' is duplicated with viewpoints[%d]", i, v.ID, j)
			}
			seenViewpointIDs[v.ID] = i
		}
		// An id-based name and an index-based name can collide (e.g. viewpoints[0] without id
		// produces viewpoint-0, and viewpoints[1] with id "0" produces viewpoint-0 too), which
		// would silently overwrite output files. Reject such conflicts on the derived name.
		name := schema.ViewpointName(v.ID, i)
		if j, ok := seenViewpointNames[name]; ok {
			return fmt.Errorf("viewpoints[%d] output name '%s' conflicts with viewpoints[%d]", i, name, j)
		}
		seenViewpointNames[name] = i
		for j, g := range v.Groups {
			if g.Name == "" {
				return fmt.Errorf("viewpoints[%d].groups[%d] name is required", i, j)
			}
			if g.Desc == "" {
				return fmt.Errorf("viewpoints[%d].groups[%d] description is required", i, j)
			}
		}
	}

	return nil
}

// LoadEnviron load environment variables.
func (c *Config) LoadEnviron() error {
	dsn := os.Getenv("TBLS_DSN")
	if dsn != "" {
		c.DSN.URL = dsn
	}
	docPath := os.Getenv("TBLS_DOC_PATH")
	if docPath != "" {
		c.DocPath = docPath
	}
	return nil
}

// LoadConfigFile load config file.
func (c *Config) LoadConfigFile(path string) (err error) {
	defer func() {
		err = errors.WithStack(err)
	}()
	if path == "" && os.Getenv("TBLS_DSN") == "" {
		var paths []string
		for _, p := range DefaultConfigFilePaths {
			if f, err := os.Stat(filepath.Join(c.root, p)); err == nil && !f.IsDir() {
				paths = append(paths, p)
			}
		}
		if len(paths) == 0 {
			return nil
		}
		if len(paths) > 1 {
			return fmt.Errorf("duplicate config file [%s]", strings.Join(paths, ", "))
		}
		path = paths[0]
	}

	if path == "" {
		return nil
	}

	fullPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	buf, err := os.ReadFile(filepath.Clean(fullPath))
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}
	c.Path = filepath.Clean(fullPath)

	return c.LoadConfig(buf)
}

// LoadConfig load config from []byte.
func (c *Config) LoadConfig(in []byte) (err error) {
	defer func() {
		err = errors.WithStack(err)
	}()
	if err := yaml.Unmarshal(expand.ExpandenvYAMLBytes(in), c); err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}
	c.MergedDict.Merge(c.Dict.Dump())
	return nil
}

// ModifySchema modify schema.Schema by config.
func (c *Config) ModifySchema(s *schema.Schema) error {
	if c.Name != "" {
		s.Name = c.Name
	}
	if c.Desc != "" {
		s.Desc = c.Desc
	}
	// set Labels
	for _, l := range c.Labels {
		s.Labels = s.Labels.Merge(l)
	}
	if err := detectPKFK(s); err != nil {
		return err
	}
	if err := c.MergeAdditionalData(s); err != nil {
		return err
	}
	var strategy *NamingStrategy
	if c.DetectVirtualRelations.Enabled {
		var err error
		strategy, err = SelectNamingStrategy(c.DetectVirtualRelations.Strategy)
		if err != nil {
			return err
		}
		warnings, err := mergeVirtualRelationRules(s, c.DetectVirtualRelations.Rules)
		for _, warning := range warnings {
			fmt.Fprintf(os.Stderr, "警告：%s\n", warning)
		}
		warningLogPath, warningLogErr := c.writeVirtualRelationWarnings(warnings)
		if warningLogErr != nil {
			fmt.Fprintf(os.Stderr, "警告：无法写入虚拟关系告警日志：%v\n", warningLogErr)
		} else if len(warnings) > 0 && warningLogPath != "" {
			fmt.Fprintf(os.Stderr, "虚拟关系告警日志：%s\n", warningLogPath)
		}
		if err != nil {
			return err
		}
	}
	if err := c.FilterTables(s); err != nil {
		return err
	}
	if c.DetectVirtualRelations.Enabled {
		mergeDetectedRelations(s, strategy)
		if !c.Format.Sort {
			sortRelationsByChildTable(s)
		}
	}
	if c.Format.Sort {
		if err := s.Sort(); err != nil {
			return err
		}
	}
	c.mergeDictFromSchema(s)
	if err := detectCardinality(s); err != nil {
		return err
	}
	if err := c.detectShowColumnsForER(s); err != nil {
		return err
	}

	// set Viewpoints
	// viewpoints should be created using as complete a schema as possible
	for _, v := range c.Viewpoints {
		cs, err := s.CloneWithoutViewpoints()
		if err != nil {
			return err
		}
		if err := cs.Filter(&schema.FilterOption{
			Include:       v.Tables,
			IncludeLabels: v.Labels,
			Distance:      v.Distance,
		}); err != nil {
			return err
		}
		if err := c.detectShowColumnsForER(cs); err != nil {
			return err
		}
		groups := []*schema.ViewpointGroup{}
		tables := lo.Map(cs.Tables, func(t *schema.Table, _ int) string {
			return t.Name
		})
		for _, g := range v.Groups {
			gt, _, err := cs.SeparateTablesThatAreIncludedOrNot(&schema.FilterOption{
				Include:       g.Tables,
				IncludeLabels: g.Labels,
			})
			if err != nil {
				return err
			}
			groups = append(groups, &schema.ViewpointGroup{
				Name:   g.Name,
				Desc:   g.Desc,
				Tables: g.Tables,
				Labels: g.Labels,
				Color:  g.Color,
			})
			left, right := lo.Difference(tables, lo.Map(gt, func(t *schema.Table, _ int) string {
				return t.Name
			}))
			if len(right) > 0 {
				return fmt.Errorf("viewpoint group '%s' has duplicate tables %v", g.Name, right)
			}
			tables = left
		}
		s.Viewpoints = s.Viewpoints.Merge(&schema.Viewpoint{
			ID:       v.ID,
			Name:     v.Name,
			Desc:     v.Desc,
			Labels:   v.Labels,
			Tables:   v.Tables,
			Distance: v.Distance,
			Groups:   groups,
			Schema:   cs,
		})
	}
	for _, v := range s.Viewpoints {
	L:
		for _, l := range v.Labels {
			for _, t := range s.Tables {
				if t.Labels.Contains(l) {
					continue L
				}
				for _, c := range t.Columns {
					if c.Labels.Contains(l) {
						continue L
					}
				}
			}
			return fmt.Errorf("viewpoint '%s' has unknown label '%s'", v.Name, l)
		}
	}
	for vi, v := range s.Viewpoints {
		// Add viewpoints to table

		for _, t := range v.Tables {
			ts, err := s.MatchTablesByName(t)
			if err != nil {
				return err
			}
			for _, tt := range ts {
				tt.Viewpoints = append(tt.Viewpoints, &schema.TableViewpoint{
					Index: vi,
					ID:    v.ID,
					Name:  v.Name,
					Desc:  v.Desc,
				})
			}
		}
	}

	return nil
}

func mergeVirtualRelationRules(s *schema.Schema, rules []VirtualRelationRule) ([]string, error) {
	explicitRelationColumns := relationColumns(s.Relations)
	mappedRelations := map[*schema.Column]*schema.Relation{}
	orderedRelations := []*schema.Relation{}
	warnings := []string{}

	for i, rule := range rules {
		if len(rule.Columns) == 0 {
			return warnings, fmt.Errorf("virtual relation rule %d: columns must not be empty", i+1)
		}
		if rule.ParentTable == "" || rule.ParentColumn == "" {
			return warnings, fmt.Errorf("virtual relation rule %d: parentTable and parentColumn are required", i+1)
		}

		parentTable, err := s.FindTableByName(rule.ParentTable)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("虚拟关系规则 %d：未找到父表 %s，已跳过该规则", i+1, rule.ParentTable))
			continue
		}
		parentColumn, err := parentTable.FindColumnByName(rule.ParentColumn)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("虚拟关系规则 %d：未找到父字段 %s.%s，已跳过该规则", i+1, parentTable.Name, rule.ParentColumn))
			continue
		}
		if !parentColumn.PK {
			warnings = append(warnings, fmt.Sprintf("虚拟关系规则 %d：父字段 %s.%s 不是主键，已跳过该规则", i+1, parentTable.Name, parentColumn.Name))
			continue
		}

		matched := false
		for _, table := range s.Tables {
			if !virtualRelationRuleMatchesTable(s, rule, table.Name) {
				continue
			}
			for _, columnName := range rule.Columns {
				column, err := table.FindColumnByName(columnName)
				if err != nil {
					continue
				}
				matched = true
				if table == parentTable {
					continue
				}
				if _, exists := explicitRelationColumns[column]; exists {
					// Database and relations: entries have the highest priority.
					continue
				}
				if !sameColumnType(column, parentColumn) {
					warnings = append(warnings, fmt.Sprintf("虚拟关系规则 %d：字段类型不匹配：%s.%s（%s）与 %s.%s（%s），已跳过该关系",
						i+1, table.Name, column.Name, column.Type, parentTable.Name, parentColumn.Name, parentColumn.Type))
					continue
				}

				relation := &schema.Relation{
					Table:         table,
					Columns:       []*schema.Column{column},
					ParentTable:   parentTable,
					ParentColumns: []*schema.Column{parentColumn},
					Def:           rule.Def,
					Virtual:       true,
				}
				if relation.Def == "" {
					relation.Def = "Mapped Relation"
				}

				if existing, exists := mappedRelations[column]; exists {
					if existing.ParentTable != parentTable || existing.ParentColumns[0] != parentColumn {
						return warnings, fmt.Errorf("conflicting virtual relation rules for %s.%s: %s.%s and %s.%s",
							table.Name, column.Name,
							existing.ParentTable.Name, existing.ParentColumns[0].Name,
							parentTable.Name, parentColumn.Name)
					}
					continue
				}

				mappedRelations[column] = relation
				orderedRelations = append(orderedRelations, relation)
			}
		}
		if !matched {
			warnings = append(warnings, fmt.Sprintf("虚拟关系规则 %d：未匹配到任何字段，已跳过该规则", i+1))
		}
	}

	for _, relation := range orderedRelations {
		column := relation.Columns[0]
		parentColumn := relation.ParentColumns[0]
		column.ParentRelations = append(column.ParentRelations, relation)
		parentColumn.ChildRelations = append(parentColumn.ChildRelations, relation)
		s.Relations = append(s.Relations, relation)
	}
	return warnings, nil
}

func virtualRelationRuleMatchesTable(s *schema.Schema, rule VirtualRelationRule, tableName string) bool {
	normalizedTableName := s.NormalizeTableName(tableName)
	if len(rule.Tables) > 0 {
		matched := false
		for _, pattern := range rule.Tables {
			if wildcard.Match(s.NormalizeTableName(pattern), normalizedTableName) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, pattern := range rule.ExcludeTables {
		if wildcard.Match(s.NormalizeTableName(pattern), normalizedTableName) {
			return false
		}
	}
	return true
}

func relationColumns(relations []*schema.Relation) map[*schema.Column]struct{} {
	columns := map[*schema.Column]struct{}{}
	for _, relation := range relations {
		for _, column := range relation.Columns {
			columns[column] = struct{}{}
		}
	}
	return columns
}

func sortRelationsByChildTable(s *schema.Schema) {
	sort.SliceStable(s.Relations, func(i, j int) bool {
		return s.Relations[i].Table.Name < s.Relations[j].Table.Name
	})
}

func (c *Config) writeVirtualRelationWarnings(warnings []string) (string, error) {
	if c.DocPath == "" {
		return "", nil
	}

	logDir := filepath.Dir(filepath.Clean(c.DocPath))
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", err
	}
	logPath := filepath.Join(logDir, VirtualRelationWarningsFileName)
	lines := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		lines = append(lines, "警告："+warning)
	}
	content := ""
	if len(lines) > 0 {
		content = strings.Join(lines, "\n") + "\n"
	}
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return logPath, nil
}

func sameColumnType(column, parentColumn *schema.Column) bool {
	return strings.EqualFold(strings.TrimSpace(column.Type), strings.TrimSpace(parentColumn.Type))
}

// MergeAdditionalData merge relations: comments: to schema.Schema.
func (c *Config) MergeAdditionalData(s *schema.Schema) error {
	if err := mergeAdditionalRelations(s, c.Relations); err != nil {
		return err
	}
	if err := mergeAdditionalComments(s, c.Comments); err != nil {
		return err
	}
	return nil
}

// FilterTables filter tables from schema.Schema using include: and exclude: and includeLabels.
func (c *Config) FilterTables(s *schema.Schema) error {
	return s.Filter(&schema.FilterOption{
		Include:       c.Include,
		Exclude:       c.Exclude,
		IncludeLabels: c.includeLabels,
		Distance:      c.Distance,
	})
}

func (c *Config) mergeDictFromSchema(s *schema.Schema) {
	if s.Driver != nil && s.Driver.Meta != nil && s.Driver.Meta.Dict != nil {
		c.MergedDict.Merge(s.Driver.Meta.Dict.Dump())
	}
}

// MaskedDSN return DSN mask password.
func (c *Config) MaskedDSN() (string, error) {
	u, err := url.Parse(c.DSN.URL)
	if err != nil {
		return c.DSN.URL, errors.WithStack(err)
	}
	_, pset := u.User.Password()
	if !pset {
		return c.DSN.URL, nil
	}
	tmp := "-----tbls-----"
	u.User = url.UserPassword(u.User.Username(), tmp)
	return strings.Replace(u.String(), tmp, "*****", 1), nil
}

func (c *Config) SchemaFilePath() string {
	return filepath.Join(c.DocPath, SchemaFileName)
}

func (c *Config) NeedToGenerateERImages() bool {
	if c.ER.Skip {
		return false
	}
	if c.ER.Format == "mermaid" {
		return false
	}
	return true
}

func (c *Config) detectShowColumnsForER(s *schema.Schema) error {
	if c.ER.ShowColumnTypes == nil {
		return nil
	}

	if !c.ER.ShowColumnTypes.Related && !c.ER.ShowColumnTypes.Primary {
		return errors.New("er.showColumnTypes: must be true at least one")
	}

	for _, t := range s.Tables {
		for _, cc := range t.Columns {
			if c.ER.ShowColumnTypes.Related && (len(cc.ChildRelations) > 0 || len(cc.ParentRelations) > 0) {
				// related
				cc.HideForER = false
			} else if c.ER.ShowColumnTypes.Primary && cc.PK {
				// primary
				cc.HideForER = false
			} else {
				cc.HideForER = true
				for _, r := range cc.ChildRelations {
					r.HideForER = true
				}
				for _, r := range cc.ParentRelations {
					r.HideForER = true
				}
			}
		}
	}

	return nil
}

func mergeAdditionalRelations(s *schema.Schema, relations []AdditionalRelation) (err error) {
	defer func() {
		err = errors.WithStack(err)
	}()
	for _, r := range relations {
		c, err := schema.ToCardinality(r.Cardinality)
		if err != nil {
			return fmt.Errorf("failed to add relation: %w", err)
		}
		pc, err := schema.ToCardinality(r.ParentCardinality)
		if err != nil {
			return fmt.Errorf("failed to add relation: %w", err)
		}
		relation := &schema.Relation{
			Cardinality:       c,
			ParentCardinality: pc,
			Virtual:           true,
		}
		if r.Def != "" {
			relation.Def = r.Def
		} else {
			relation.Def = "Additional Relation"
		}
		relation.Table, err = s.FindTableByName(r.Table)
		if err != nil {
			return fmt.Errorf("failed to add relation: %w", err)
		}
		for _, c := range r.Columns {
			column, err := relation.Table.FindColumnByName(c)
			if err != nil {
				return fmt.Errorf("failed to add relation: %w", err)
			}
			relation.Columns = append(relation.Columns, column)
			column.ParentRelations = append(column.ParentRelations, relation)
		}
		relation.ParentTable, err = s.FindTableByName(r.ParentTable)
		if err != nil {
			return fmt.Errorf("failed to add relation: %w", err)
		}
		for _, c := range r.ParentColumns {
			column, err := relation.ParentTable.FindColumnByName(c)
			if err != nil {
				return fmt.Errorf("failed to add relation: %w", err)
			}
			relation.ParentColumns = append(relation.ParentColumns, column)
			column.ChildRelations = append(column.ChildRelations, relation)
		}

		if r.Override {
			cr, err := s.FindRelation(relation.Columns, relation.ParentColumns)
			if err != nil {
				s.Relations = append(s.Relations, relation)
			} else {
				cr.Virtual = true
				cr.Def = r.Def
				cr.Cardinality, err = schema.ToCardinality(r.Cardinality)
				if err != nil {
					return fmt.Errorf("failed to add relation: %w", err)
				}
				cr.ParentCardinality, err = schema.ToCardinality(r.ParentCardinality)
				if err != nil {
					return fmt.Errorf("failed to add relation: %w", err)
				}
			}
		} else {
			s.Relations = append(s.Relations, relation)
		}
	}
	return nil
}

func mergeAdditionalComments(s *schema.Schema, comments []AdditionalComment) (err error) {
	defer func() {
		err = errors.WithStack(err)
	}()
	for _, c := range comments {
		table, err := s.FindTableByName(c.Table)
		if err != nil {
			return fmt.Errorf("failed to add table comment: %w", err)
		}
		if c.TableComment != "" {
			table.Comment = c.TableComment
		}
		if len(c.Labels) > 0 {
			for _, l := range c.Labels {
				table.Labels = table.Labels.Merge(l)
			}
		}
		for c, comment := range c.ColumnComments {
			column, err := table.FindColumnByName(c)
			if err != nil {
				return fmt.Errorf("failed to add column comment: %w", err)
			}
			column.Comment = comment
		}
		for c, labels := range c.ColumnLabels {
			column, err := table.FindColumnByName(c)
			if err != nil {
				return fmt.Errorf("failed to add column comment: %w", err)
			}
			for _, l := range labels {
				column.Labels = column.Labels.Merge(l)
			}
		}
		for i, comment := range c.IndexComments {
			index, err := table.FindIndexByName(i)
			if err != nil {
				return fmt.Errorf("failed to add index comment: %w", err)
			}
			index.Comment = comment
		}
		for c, comment := range c.ConstraintComments {
			constraint, err := table.FindConstraintByName(c)
			if err != nil {
				return fmt.Errorf("failed to add constraint comment: %w", err)
			}
			constraint.Comment = comment
		}
		for t, comment := range c.TriggerComments {
			trigger, err := table.FindTriggerByName(t)
			if err != nil {
				return fmt.Errorf("failed to add trigger comment: %w", err)
			}
			trigger.Comment = comment
		}
	}
	return nil
}

func mergeDetectedRelations(s *schema.Schema, strategy *NamingStrategy) {
	explicitRelationColumns := relationColumns(s.Relations)

	for _, t := range s.Tables {
		for _, c := range t.Columns {
			if _, exists := explicitRelationColumns[c]; exists {
				// Relations loaded from the database or configured in relations: take
				// precedence over heuristic detection for the same child column.
				continue
			}

			relation := &schema.Relation{
				Virtual: true,
				Def:     "Detected Relation",
				Table:   t,
			}

			parentTable, parentColumn, ok := findDetectedRelationParent(s, t, c, strategy)
			if !ok {
				continue
			}

			relation.ParentTable = parentTable
			relation.Columns = append(relation.Columns, c)
			relation.ParentColumns = append(relation.ParentColumns, parentColumn)

			if _, err := s.FindRelation(relation.Columns, relation.ParentColumns); err == nil {
				// If the relation already exists, do not create a new virtual relation.
				continue
			}

			c.ParentRelations = append(c.ParentRelations, relation)
			parentColumn.ChildRelations = append(parentColumn.ChildRelations, relation)
			s.Relations = append(s.Relations, relation)
		}
	}
}

func findDetectedRelationParent(s *schema.Schema, childTable *schema.Table, childColumn *schema.Column, strategy *NamingStrategy) (*schema.Table, *schema.Column, bool) {
	parentColumnName := strategy.ParentColumnName(childColumn.Name)
	parentTable, err := s.FindTableByName(strategy.ParentTableNameFor(childTable.Name, childColumn.Name))
	if err == nil && parentTable != childTable {
		if parentColumn, err := parentTable.FindColumnByName(parentColumnName); err == nil && validDetectedParentColumn(childColumn, parentColumn, strategy) {
			return parentTable, parentColumn, true
		}
	}

	if !strategy.AllowUniqueTableSuffix {
		return nil, nil, false
	}
	entityName := fmsEntityName(childColumn.Name)
	if entityName == "" {
		return nil, nil, false
	}

	var matchedTable *schema.Table
	var matchedColumn *schema.Column
	for _, candidate := range s.Tables {
		if candidate == childTable {
			continue
		}
		candidateName := candidate.Name
		if index := strings.LastIndex(candidateName, "."); index >= 0 {
			candidateName = candidateName[index+1:]
		}
		if !strings.HasSuffix(strings.ToLower(candidateName), "_"+strings.ToLower(entityName)) {
			continue
		}
		parentColumn, err := candidate.FindColumnByName(parentColumnName)
		if err != nil || !validDetectedParentColumn(childColumn, parentColumn, strategy) {
			continue
		}
		if matchedTable != nil {
			// Multiple valid cross-module candidates are ambiguous.
			return nil, nil, false
		}
		matchedTable = candidate
		matchedColumn = parentColumn
	}
	if matchedTable == nil {
		return nil, nil, false
	}
	return matchedTable, matchedColumn, true
}

func validDetectedParentColumn(childColumn, parentColumn *schema.Column, strategy *NamingStrategy) bool {
	if strategy.RequireParentPK && !parentColumn.PK {
		return false
	}
	return !strategy.RequireSameType || sameColumnType(childColumn, parentColumn)
}

func matchLength(s []string, e string) (int, bool) {
	for _, v := range s {
		if wildcard.Match(v, e) {
			return len(strings.ReplaceAll(v, "*", "")), true
		}
	}
	return 0, false
}

// This function should be applied to the completed schema.
func detectCardinality(s *schema.Schema) error {
	for _, r := range s.Relations {
		// child
		if r.Cardinality == schema.UnknownCardinality {
			unique := false
			columns := []string{}
			for _, c := range r.Columns {
				columns = append(columns, c.Name)
			}
		LL:
			for _, c := range r.Table.Constraints {
				if len(columns) != len(c.Columns) {
					continue
				}
				for _, cc := range c.Columns {
					if !lo.Contains(columns, cc) {
						continue LL
					}
				}
				if strings.Contains(strings.ToUpper(c.Def), "UNIQUE") || strings.Contains(strings.ToUpper(c.Def), "PRIMARY KEY") {
					unique = true
				}
			}
			if unique {
				r.Cardinality = schema.ZeroOrOne
			} else {
				r.Cardinality = schema.ZeroOrMore
			}
		}

		// parent
		if r.ParentCardinality == schema.UnknownCardinality {
			// whether the child columns are nullable or not.
			nullable := true
			for _, c := range r.Columns {
				if !c.Nullable {
					nullable = false
				}
			}

			if nullable {
				r.ParentCardinality = schema.ZeroOrOne
			} else {
				r.ParentCardinality = schema.ExactlyOne
			}
		}
	}
	return nil
}

func detectPKFK(s *schema.Schema) error {
	for _, t := range s.Tables {
		// PRIMARY KEY
		for _, i := range t.Indexes {
			if !strings.Contains(i.Def, "PRIMARY") {
				continue
			}
			for _, c := range i.Columns {
				column, err := t.FindColumnByName(c)
				if err != nil {
					return err
				}
				column.PK = true
			}
		}
		// Foreign Key (Relations)
		for _, c := range t.Columns {
			if len(c.ParentRelations) > 0 && !c.PK {
				c.FK = true
			}
		}
	}
	return nil
}

func match(s []string, e string) bool {
	_, m := matchLength(s, e)
	return m
}
