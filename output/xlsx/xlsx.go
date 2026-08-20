package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/k1LoW/errors"
	"github.com/k1LoW/tbls/config"
	"github.com/k1LoW/tbls/schema"
	"github.com/loadoff/excl"
)

const (
	moduleSheetName     = "模块清单"
	tableSheetName      = "表清单"
	columnSheetName     = "字段清单"
	relationSheetName   = "关系清单"
	constraintSheetName = "索引与约束"
)

var (
	headerFont = excl.Font{Bold: true, Color: "FFFFFFFF"}
	linkFont   = excl.Font{Color: "FF0563C1", Underline: true}
	headerFill = "FF1F4E78"
	groupFills = []string{"FFD9EAF7", "FFE2F0D9"}
)

// Xlsx struct.
type Xlsx struct {
	config *config.Config
	links  map[string][]internalLink
}

type internalLink struct {
	Cell        string
	TargetSheet string
	TargetCell  string
	Label       string
}

type moduleEntry struct {
	Code        string
	Name        string
	Description string
	TableCount  int
	TableRow    int
}

type tableEntry struct {
	ModuleCode string
	ModuleName string
	Table      *schema.Table
	TableRow   int
	ColumnRow  int
}

type workbookLayout struct {
	Modules []moduleEntry
	Tables  []tableEntry
}

// New return Xlsx.
func New(c *config.Config) *Xlsx {
	return &Xlsx{config: c}
}

// OutputSchema outputs searchable, centralized lists instead of one sheet per table.
func (x *Xlsx) OutputSchema(wr io.Writer, s *schema.Schema) error {
	w, err := excl.Create()
	if err != nil {
		return err
	}
	w.SetForceFormulaRecalculation(true)
	x.links = map[string][]internalLink{}

	layout := x.buildLayout(s)
	for _, create := range []func(*excl.Workbook, *schema.Schema, workbookLayout) error{
		x.createModuleSheet,
		x.createTableListSheet,
		x.createColumnListSheet,
		x.createRelationListSheet,
		x.createConstraintListSheet,
	} {
		if err := create(w, s, layout); err != nil {
			return err
		}
	}

	tf, err := os.CreateTemp("", "tbls-*.xlsx")
	if err != nil {
		return err
	}
	path := tf.Name()
	if err := tf.Close(); err != nil {
		return err
	}
	defer func() { _ = os.Remove(path) }()
	if err := w.Save(path); err != nil {
		return err
	}
	if err := decorateWorkbook(path, x.links, workbookFilterRanges(s, layout)); err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Clean(path)) //nolint:gosec
	if err != nil {
		return err
	}
	_, err = wr.Write(b)
	return err
}

// OutputTable outputs the same searchable layout for a single table.
func (x *Xlsx) OutputTable(wr io.Writer, t *schema.Table) error {
	return x.OutputSchema(wr, &schema.Schema{
		Name:      t.Name,
		Tables:    []*schema.Table{t},
		Relations: collectTableRelations(t),
	})
}

func (x *Xlsx) buildLayout(s *schema.Schema) workbookLayout {
	tables := append([]*schema.Table(nil), s.Tables...)
	sort.SliceStable(tables, func(i, j int) bool {
		mi := x.moduleCode(tables[i].Name)
		mj := x.moduleCode(tables[j].Name)
		if mi != mj {
			if mi == "other" {
				return false
			}
			if mj == "other" {
				return true
			}
			return mi < mj
		}
		return tables[i].Name < tables[j].Name
	})

	layout := workbookLayout{}
	moduleIndexes := map[string]int{}
	tableRow := 2
	columnRow := 2
	for _, t := range tables {
		code := x.moduleCode(t.Name)
		moduleIndex, ok := moduleIndexes[code]
		if !ok {
			name, desc := moduleDetails(code, t)
			moduleIndex = len(layout.Modules)
			moduleIndexes[code] = moduleIndex
			layout.Modules = append(layout.Modules, moduleEntry{
				Code:        code,
				Name:        name,
				Description: desc,
				TableRow:    tableRow,
			})
		}
		layout.Modules[moduleIndex].TableCount++
		layout.Tables = append(layout.Tables, tableEntry{
			ModuleCode: code,
			ModuleName: layout.Modules[moduleIndex].Name,
			Table:      t,
			TableRow:   tableRow,
			ColumnRow:  columnRow,
		})
		tableRow++
		if len(t.Columns) == 0 {
			columnRow++
		} else {
			columnRow += len(t.Columns)
		}
	}
	return layout
}

func (x *Xlsx) moduleCode(tableName string) string {
	separator := x.config.ModuleViewpoints.Separator
	if separator == "" {
		separator = x.config.TableDirectories.Separator
	}
	if separator == "" {
		separator = "_"
	}
	name := tableName
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.Index(name, separator); i > 0 {
		return name[:i]
	}
	return "other"
}

func moduleDetails(code string, t *schema.Table) (string, string) {
	for _, viewpoint := range t.Viewpoints {
		if viewpoint == nil {
			continue
		}
		if strings.EqualFold(viewpoint.ID, "module-"+code) && viewpoint.Name != "" {
			return viewpoint.Name, viewpoint.Desc
		}
	}
	if code == "other" {
		return "其他模块", "未匹配模块前缀的表"
	}
	return strings.ToUpper(code) + " 模块", fmt.Sprintf("按表名前缀 %s 归类", code)
}

func (x *Xlsx) createModuleSheet(w *excl.Workbook, _ *schema.Schema, layout workbookLayout) error {
	sheet, err := w.OpenSheet(moduleSheetName)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() { _ = sheet.Close() }()

	setWidths(sheet, []float64{22, 14, 10, 42})
	setHeader(sheet, 1, []string{"模块名称", "模块编码", "表数量", "说明"})
	for i, module := range layout.Modules {
		row := i + 2
		x.setInternalLink(sheet, moduleSheetName, row, 1, tableSheetName, fmt.Sprintf("A%d", module.TableRow), module.Name)
		setString(sheet, row, 2, module.Code)
		setNumber(sheet, row, 3, module.TableCount)
		setWrappedString(sheet, row, 4, module.Description)
		shadeCell(sheet, row, 1, i)
	}
	return nil
}

func (x *Xlsx) createTableListSheet(w *excl.Workbook, _ *schema.Schema, layout workbookLayout) error {
	sheet, err := w.OpenSheet(tableSheetName)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() { _ = sheet.Close() }()

	setWidths(sheet, []float64{18, 28, 40, 12, 10, 12, 12})
	setHeader(sheet, 1, []string{"模块", "表名", "表说明", "类型", "字段数", "关联父表数", "被关联表数"})
	moduleIndex := moduleOrder(layout.Modules)
	for _, entry := range layout.Tables {
		parents, children := tableRelationCounts(entry.Table)
		setString(sheet, entry.TableRow, 1, entry.ModuleName)
		x.setInternalLink(sheet, tableSheetName, entry.TableRow, 2, columnSheetName, fmt.Sprintf("B%d", entry.ColumnRow), entry.Table.Name)
		setWrappedString(sheet, entry.TableRow, 3, entry.Table.Comment)
		setString(sheet, entry.TableRow, 4, entry.Table.Type)
		setNumber(sheet, entry.TableRow, 5, len(entry.Table.Columns))
		setNumber(sheet, entry.TableRow, 6, parents)
		setNumber(sheet, entry.TableRow, 7, children)
		shadeCell(sheet, entry.TableRow, 1, moduleIndex[entry.ModuleCode])
	}
	return nil
}

func (x *Xlsx) createColumnListSheet(w *excl.Workbook, _ *schema.Schema, layout workbookLayout) error {
	sheet, err := w.OpenSheet(columnSheetName)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() { _ = sheet.Close() }()

	setWidths(sheet, []float64{18, 28, 8, 26, 20, 8, 22, 8, 8, 38, 48})
	setHeader(sheet, 1, []string{"模块", "表名", "序号", "字段名", "数据类型", "可空", "默认值", "主键", "关联字段", "字段说明", "关系描述"})
	moduleIndex := moduleOrder(layout.Modules)
	for _, entry := range layout.Tables {
		if len(entry.Table.Columns) == 0 {
			setString(sheet, entry.ColumnRow, 1, entry.ModuleName)
			setString(sheet, entry.ColumnRow, 2, entry.Table.Name)
			setString(sheet, entry.ColumnRow, 4, "（无字段）")
			shadeCell(sheet, entry.ColumnRow, 1, moduleIndex[entry.ModuleCode])
			continue
		}
		for i, column := range entry.Table.Columns {
			row := entry.ColumnRow + i
			setString(sheet, row, 1, entry.ModuleName)
			setString(sheet, row, 2, entry.Table.Name)
			setNumber(sheet, row, 3, i+1)
			setString(sheet, row, 4, column.Name)
			setString(sheet, row, 5, column.Type)
			setString(sheet, row, 6, yesNo(column.Nullable))
			if column.Default.Valid {
				setString(sheet, row, 7, column.Default.String)
			}
			setString(sheet, row, 8, yesNo(column.PK))
			setString(sheet, row, 9, yesNo(len(column.ParentRelations)+len(column.ChildRelations) > 0))
			setWrappedString(sheet, row, 10, column.Comment)
			setWrappedString(sheet, row, 11, relationDescription(column))
			shadeCell(sheet, row, 1, moduleIndex[entry.ModuleCode])
		}
	}
	return nil
}

func (x *Xlsx) createRelationListSheet(w *excl.Workbook, s *schema.Schema, layout workbookLayout) error {
	sheet, err := w.OpenSheet(relationSheetName)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() { _ = sheet.Close() }()

	setWidths(sheet, []float64{18, 28, 24, 18, 28, 24, 12, 12, 14, 48})
	setHeader(sheet, 1, []string{"子模块", "子表", "子字段", "父模块", "父表", "父字段", "子端基数", "父端基数", "关系类型", "关系说明"})
	tableRows, columnRows := layoutRows(layout)
	relations := append([]*schema.Relation(nil), s.Relations...)
	sort.SliceStable(relations, func(i, j int) bool { return relationSortKey(relations[i]) < relationSortKey(relations[j]) })
	row := 2
	for _, relation := range relations {
		if relation == nil || relation.Table == nil || relation.ParentTable == nil {
			continue
		}
		childModule := x.moduleCode(relation.Table.Name)
		parentModule := x.moduleCode(relation.ParentTable.Name)
		setString(sheet, row, 1, moduleNameByCode(layout.Modules, childModule))
		x.setTableOrTextLink(sheet, relationSheetName, row, 2, relation.Table, tableRows, columnRows)
		setString(sheet, row, 3, joinColumnNames(relation.Columns))
		setString(sheet, row, 4, moduleNameByCode(layout.Modules, parentModule))
		x.setTableOrTextLink(sheet, relationSheetName, row, 5, relation.ParentTable, tableRows, columnRows)
		setString(sheet, row, 6, joinColumnNames(relation.ParentColumns))
		setString(sheet, row, 7, cardinalityText(relation.Cardinality))
		setString(sheet, row, 8, cardinalityText(relation.ParentCardinality))
		if relation.Virtual {
			setString(sheet, row, 9, "虚拟关系")
		} else {
			setString(sheet, row, 9, "数据库外键")
		}
		setWrappedString(sheet, row, 10, relation.Def)
		row++
	}
	return nil
}

func (x *Xlsx) createConstraintListSheet(w *excl.Workbook, _ *schema.Schema, layout workbookLayout) error {
	sheet, err := w.OpenSheet(constraintSheetName)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() { _ = sheet.Close() }()

	setWidths(sheet, []float64{18, 28, 12, 30, 18, 60, 38})
	setHeader(sheet, 1, []string{"模块", "表名", "类别", "名称", "类型", "定义", "说明"})
	row := 2
	for _, entry := range layout.Tables {
		for _, constraint := range entry.Table.Constraints {
			x.setConstraintRow(sheet, row, entry, "约束", constraint.Name, constraint.Type, constraint.Def, constraint.Comment)
			row++
		}
		for _, index := range entry.Table.Indexes {
			x.setConstraintRow(sheet, row, entry, "索引", index.Name, "", index.Def, index.Comment)
			row++
		}
		for _, trigger := range entry.Table.Triggers {
			x.setConstraintRow(sheet, row, entry, "触发器", trigger.Name, "", trigger.Def, trigger.Comment)
			row++
		}
	}
	return nil
}

func (x *Xlsx) setConstraintRow(sheet *excl.Sheet, row int, entry tableEntry, category, name, typ, def, comment string) {
	setString(sheet, row, 1, entry.ModuleName)
	x.setInternalLink(sheet, constraintSheetName, row, 2, columnSheetName, fmt.Sprintf("B%d", entry.ColumnRow), entry.Table.Name)
	setString(sheet, row, 3, category)
	setString(sheet, row, 4, name)
	setString(sheet, row, 5, typ)
	setWrappedString(sheet, row, 6, def)
	setWrappedString(sheet, row, 7, comment)
}

func relationDescription(column *schema.Column) string {
	descriptions := make([]string, 0, len(column.ParentRelations)+len(column.ChildRelations))
	for _, relation := range column.ParentRelations {
		if relation.ParentTable == nil {
			continue
		}
		target := correspondingColumnNames(column, relation.Columns, relation.ParentColumns)
		if target != "" {
			descriptions = append(descriptions, fmt.Sprintf("关联%s表%s字段", relation.ParentTable.Name, target))
		}
	}
	for _, relation := range column.ChildRelations {
		if relation.Table == nil {
			continue
		}
		source := correspondingColumnNames(column, relation.ParentColumns, relation.Columns)
		if source != "" {
			descriptions = append(descriptions, fmt.Sprintf("被%s表%s字段关联", relation.Table.Name, source))
		}
	}
	sort.Strings(descriptions)
	return strings.Join(uniqueStrings(descriptions), "；")
}

func correspondingColumnNames(column *schema.Column, columns, related []*schema.Column) string {
	for i, candidate := range columns {
		if candidate != nil && (candidate == column || candidate.Name == column.Name) {
			if i < len(related) && related[i] != nil {
				return related[i].Name
			}
		}
	}
	return joinColumnNames(related)
}

func joinColumnNames(columns []*schema.Column) string {
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		if column != nil {
			names = append(names, column.Name)
		}
	}
	return strings.Join(names, "、")
}

func tableRelationCounts(t *schema.Table) (int, int) {
	parents := map[string]struct{}{}
	children := map[string]struct{}{}
	for _, column := range t.Columns {
		for _, relation := range column.ParentRelations {
			if relation.ParentTable != nil {
				parents[relation.ParentTable.Name] = struct{}{}
			}
		}
		for _, relation := range column.ChildRelations {
			if relation.Table != nil {
				children[relation.Table.Name] = struct{}{}
			}
		}
	}
	return len(parents), len(children)
}

func collectTableRelations(t *schema.Table) []*schema.Relation {
	seen := map[*schema.Relation]struct{}{}
	relations := []*schema.Relation{}
	for _, column := range t.Columns {
		all := append(append([]*schema.Relation{}, column.ParentRelations...), column.ChildRelations...)
		for _, relation := range all {
			if _, ok := seen[relation]; ok {
				continue
			}
			seen[relation] = struct{}{}
			relations = append(relations, relation)
		}
	}
	return relations
}

func layoutRows(layout workbookLayout) (map[*schema.Table]int, map[*schema.Table]int) {
	tableRows := map[*schema.Table]int{}
	columnRows := map[*schema.Table]int{}
	for _, entry := range layout.Tables {
		tableRows[entry.Table] = entry.TableRow
		columnRows[entry.Table] = entry.ColumnRow
	}
	return tableRows, columnRows
}

func (x *Xlsx) setTableOrTextLink(sheet *excl.Sheet, sheetName string, row, col int, table *schema.Table, tableRows, columnRows map[*schema.Table]int) {
	if table == nil {
		return
	}
	if target, ok := columnRows[table]; ok {
		x.setInternalLink(sheet, sheetName, row, col, columnSheetName, fmt.Sprintf("B%d", target), table.Name)
		return
	}
	if target, ok := tableRows[table]; ok {
		x.setInternalLink(sheet, sheetName, row, col, tableSheetName, fmt.Sprintf("B%d", target), table.Name)
		return
	}
	setString(sheet, row, col, table.Name)
}

func moduleOrder(modules []moduleEntry) map[string]int {
	order := map[string]int{}
	for i, module := range modules {
		order[module.Code] = i
	}
	return order
}

func moduleNameByCode(modules []moduleEntry, code string) string {
	for _, module := range modules {
		if module.Code == code {
			return module.Name
		}
	}
	if code == "other" {
		return "其他模块"
	}
	return strings.ToUpper(code) + " 模块"
}

func relationSortKey(relation *schema.Relation) string {
	if relation == nil || relation.Table == nil || relation.ParentTable == nil {
		return ""
	}
	return relation.Table.Name + "\x00" + joinColumnNames(relation.Columns) + "\x00" + relation.ParentTable.Name
}

func cardinalityText(cardinality schema.Cardinality) string {
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
		return "未知"
	}
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func yesNo(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func setWidths(sheet *excl.Sheet, widths []float64) {
	for i, width := range widths {
		sheet.SetColWidth(width, i+1)
	}
}

func setHeader(sheet *excl.Sheet, row int, values []string) {
	for i, value := range values {
		setString(sheet, row, i+1, value).
			SetFont(headerFont).
			SetBackgroundColor(headerFill).
			SetStyle(&excl.Style{Vertical: "center", Wrap: 1})
	}
	sheet.GetRow(row).SetHeight(24)
}

func (x *Xlsx) setInternalLink(sheet *excl.Sheet, sheetName string, row, col int, targetSheet, targetCell, label string) {
	setString(sheet, row, col, label).SetFont(linkFont)
	x.links[sheetName] = append(x.links[sheetName], internalLink{
		Cell:        fmt.Sprintf("%s%d", excl.ColStringPosition(col), row),
		TargetSheet: targetSheet,
		TargetCell:  targetCell,
		Label:       label,
	})
}

func decorateWorkbook(path string, links map[string][]internalLink, filterRanges map[string]string) error {
	data, err := os.ReadFile(filepath.Clean(path)) //nolint:gosec
	if err != nil {
		return err
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	sheetFiles := map[string]string{
		moduleSheetName:     "xl/worksheets/sheet1.xml",
		tableSheetName:      "xl/worksheets/sheet2.xml",
		columnSheetName:     "xl/worksheets/sheet3.xml",
		relationSheetName:   "xl/worksheets/sheet4.xml",
		constraintSheetName: "xl/worksheets/sheet5.xml",
	}
	linksByFile := map[string][]internalLink{}
	filtersByFile := map[string]string{}
	for sheetName, sheetLinks := range links {
		linksByFile[sheetFiles[sheetName]] = sheetLinks
	}
	for sheetName, filterRange := range filterRanges {
		filtersByFile[sheetFiles[sheetName]] = filterRange
	}

	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range reader.File {
		input, err := file.Open()
		if err != nil {
			return err
		}
		content, err := io.ReadAll(input)
		_ = input.Close()
		if err != nil {
			return err
		}
		if filterRange := filtersByFile[file.Name]; filterRange != "" {
			content, err = injectWorksheetFeatures(content, linksByFile[file.Name], filterRange)
			if err != nil {
				return err
			}
		}
		header := file.FileHeader
		entry, err := writer.CreateHeader(&header)
		if err != nil {
			return err
		}
		if _, err := entry.Write(content); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, output.Bytes(), 0600)
}

func injectWorksheetFeatures(worksheet []byte, links []internalLink, filterRange string) ([]byte, error) {
	emptyView := []byte(`<sheetView workbookViewId="0"></sheetView>`)
	frozenView := []byte(`<sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/><selection pane="bottomLeft" activeCell="A2" sqref="A2"/></sheetView>`)
	worksheet = bytes.Replace(worksheet, emptyView, frozenView, 1)

	closingTag := []byte("</sheetData>")
	position := bytes.LastIndex(worksheet, closingTag)
	if position < 0 {
		return nil, errors.New("invalid worksheet XML: sheetData closing tag not found")
	}
	position += len(closingTag)
	var fragment strings.Builder
	fragment.WriteString(`<autoFilter ref="`)
	fragment.WriteString(xmlEscape(filterRange))
	fragment.WriteString(`"/>`)
	if len(links) > 0 {
		fragment.WriteString("<hyperlinks>")
		for _, link := range links {
			fragment.WriteString(`<hyperlink ref="`)
			fragment.WriteString(xmlEscape(link.Cell))
			fragment.WriteString(`" location="'`)
			fragment.WriteString(xmlEscape(strings.ReplaceAll(link.TargetSheet, "'", "''")))
			fragment.WriteString(`'!`)
			fragment.WriteString(xmlEscape(link.TargetCell))
			fragment.WriteString(`" display="`)
			fragment.WriteString(xmlEscape(link.Label))
			fragment.WriteString(`"/>`)
		}
		fragment.WriteString("</hyperlinks>")
	}
	result := make([]byte, 0, len(worksheet)+fragment.Len())
	result = append(result, worksheet[:position]...)
	result = append(result, fragment.String()...)
	result = append(result, worksheet[position:]...)
	return result, nil
}

func workbookFilterRanges(s *schema.Schema, layout workbookLayout) map[string]string {
	lastColumnRow := 1
	if len(layout.Tables) > 0 {
		last := layout.Tables[len(layout.Tables)-1]
		lastColumnRow = last.ColumnRow + len(last.Table.Columns) - 1
		if len(last.Table.Columns) == 0 {
			lastColumnRow = last.ColumnRow
		}
	}
	validRelations := 0
	for _, relation := range s.Relations {
		if relation != nil && relation.Table != nil && relation.ParentTable != nil {
			validRelations++
		}
	}
	constraintRows := 0
	for _, entry := range layout.Tables {
		constraintRows += len(entry.Table.Constraints) + len(entry.Table.Indexes) + len(entry.Table.Triggers)
	}
	return map[string]string{
		moduleSheetName:     fmt.Sprintf("A1:D%d", max(1, len(layout.Modules)+1)),
		tableSheetName:      fmt.Sprintf("A1:G%d", max(1, len(layout.Tables)+1)),
		columnSheetName:     fmt.Sprintf("A1:K%d", lastColumnRow),
		relationSheetName:   fmt.Sprintf("A1:J%d", max(1, validRelations+1)),
		constraintSheetName: fmt.Sprintf("A1:G%d", max(1, constraintRows+1)),
	}
}

func xmlEscape(value string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}

func shadeCell(sheet *excl.Sheet, row, col, groupIndex int) {
	sheet.GetRow(row).GetCell(col).SetBackgroundColor(groupFills[groupIndex%len(groupFills)])
}

func setString(sheet *excl.Sheet, rowNo, colNo int, value string) *excl.Cell {
	cell := sheet.GetRow(rowNo).GetCell(colNo)
	if value == "" {
		return cell
	}
	return cell.SetString(value)
}

func setWrappedString(sheet *excl.Sheet, rowNo, colNo int, value string) *excl.Cell {
	return setString(sheet, rowNo, colNo, value).SetStyle(&excl.Style{Vertical: "top", Wrap: 1})
}

func setNumber(sheet *excl.Sheet, rowNo, colNo int, value int) *excl.Cell {
	return sheet.GetRow(rowNo).SetNumber(value, colNo)
}
