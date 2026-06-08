package services

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/repositories"
)

// ReportTemplateService renders a report's receipts into a group's uploaded
// custom template (xlsx or csv), as an alternative to the built-in default
// workbook. For xlsx it writes rows into the uploaded workbook, locating the
// write region by marker cells first, then by header matching, leaving the rest
// of the workbook (formulas, other sheets) intact. For csv it emits rows in the
// column order declared by the uploaded file's header row.
type ReportTemplateService struct {
	BaseService
	xlsxService ReportXlsxService
}

func NewReportTemplateService(tx *gorm.DB) ReportTemplateService {
	return ReportTemplateService{
		BaseService: BaseService{DB: repositories.GetDB(), TX: tx},
		xlsxService: NewReportXlsxService(tx),
	}
}

// fieldForHeader maps a normalised template header to a function that produces
// the cell value for a given row. Header matching is case-insensitive and
// matches on contained keywords so "Merchant", "Vendor", "Supplier" all map to
// the merchant name.
type headerMatcher struct {
	keywords []string
	value    func(r xlsxRow, home string) string
}

func reportHeaderMatchers() []headerMatcher {
	return []headerMatcher{
		{[]string{"date"}, func(r xlsxRow, _ string) string { return r.date.Format("2006-01-02") }},
		{[]string{"merchant", "vendor", "supplier", "payee", "description"}, func(r xlsxRow, _ string) string { return r.merchant }},
		{[]string{"category"}, func(r xlsxRow, _ string) string { return r.category }},
		{[]string{"orig", "original"}, func(r xlsxRow, _ string) string {
			if r.origAmount != nil {
				return r.origAmount.String()
			}
			return ""
		}},
		{[]string{"currency", "cur"}, func(r xlsxRow, home string) string {
			if r.isForeign {
				return r.currency
			}
			return home
		}},
		{[]string{"vat", "tax"}, func(r xlsxRow, _ string) string { return r.tax.String() }},
		{[]string{"total", "amount", "gross", "net"}, func(r xlsxRow, _ string) string { return r.total.String() }},
		{[]string{"review", "note", "flag"}, func(r xlsxRow, _ string) string { return r.review }},
		{[]string{"#", "no", "ref", "index"}, func(r xlsxRow, _ string) string { return strconv.Itoa(r.index) }},
	}
}

func matchHeader(header string) *headerMatcher {
	norm := strings.ToLower(strings.TrimSpace(header))
	if norm == "" {
		return nil
	}
	for _, m := range reportHeaderMatchers() {
		for _, kw := range m.keywords {
			if strings.Contains(norm, kw) {
				mm := m
				return &mm
			}
		}
	}
	return nil
}

// BuildFromTemplate renders the report into the group's uploaded template file.
func (service ReportTemplateService) BuildFromTemplate(report models.Report, settings models.GroupReceiptSettings, templatePath string) ([]byte, string, error) {
	home := settings.HomeCurrency
	if home == "" {
		home = "GBP"
	}
	rows := service.xlsxService.buildRows(report, settings, home)

	switch strings.ToLower(settings.ReportTemplateType) {
	case "csv":
		data, err := service.renderCsv(rows, templatePath, home)
		return data, "csv", err
	case "xlsx":
		data, err := service.renderXlsx(rows, templatePath, home)
		return data, "xlsx", err
	default:
		return nil, "", fmt.Errorf("unsupported template type %q", settings.ReportTemplateType)
	}
}

func (service ReportTemplateService) renderCsv(rows []xlsxRow, templatePath string, home string) ([]byte, error) {
	raw, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("template csv has no header row")
	}
	header := records[0]
	matchers := make([]*headerMatcher, len(header))
	for i, h := range header {
		matchers[i] = matchHeader(h)
	}

	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	for _, r := range rows {
		record := make([]string, len(header))
		for i, m := range matchers {
			if m != nil {
				record[i] = m.value(r, home)
			}
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (service ReportTemplateService) renderXlsx(rows []xlsxRow, templatePath string, home string) ([]byte, error) {
	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	if sheet == "" {
		return nil, fmt.Errorf("template workbook has no sheets")
	}

	startRow, colMatchers, err := service.locateWriteRegion(f, sheet)
	if err != nil {
		return nil, err
	}

	for i, r := range rows {
		rowNum := startRow + i
		for col, m := range colMatchers {
			if m == nil {
				continue
			}
			cell, cellErr := excelize.CoordinatesToCellName(col+1, rowNum)
			if cellErr != nil {
				return nil, cellErr
			}
			if err := f.SetCellValue(sheet, cell, m.value(r, home)); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// locateWriteRegion finds where to begin writing rows and which column maps to
// which field. It first looks for a marker cell containing "{{rows}}" (data
// begins on that row, and the header is the row above). If no marker is found,
// it falls back to scanning for a recognisable header row and begins on the
// next row.
func (service ReportTemplateService) locateWriteRegion(f *excelize.File, sheet string) (int, []*headerMatcher, error) {
	cols, err := f.GetCols(sheet)
	if err != nil {
		return 0, nil, err
	}
	rowsData, err := f.GetRows(sheet)
	if err != nil {
		return 0, nil, err
	}

	// Marker search: a cell whose value is "{{rows}}".
	for rIdx, row := range rowsData {
		for cIdx, val := range row {
			if strings.TrimSpace(strings.ToLower(val)) == "{{rows}}" {
				headerRow := rIdx - 1
				if headerRow < 0 {
					headerRow = rIdx
				}
				matchers := service.matchersForHeaderRow(rowsData, headerRow, len(cols))
				// Clear the marker cell so it isn't left in the output.
				if cell, cErr := excelize.CoordinatesToCellName(cIdx+1, rIdx+1); cErr == nil {
					_ = f.SetCellValue(sheet, cell, "")
				}
				return rIdx + 1, matchers, nil
			}
		}
	}

	// Header-match fallback: find the first row where at least two cells map.
	for rIdx, row := range rowsData {
		matched := 0
		for _, val := range row {
			if matchHeader(val) != nil {
				matched++
			}
		}
		if matched >= 2 {
			matchers := service.matchersForHeaderRow(rowsData, rIdx, len(cols))
			return rIdx + 2, matchers, nil
		}
	}

	return 0, nil, fmt.Errorf("could not locate a header row or {{rows}} marker in the template; add a header row (Date, Merchant, Total, ...) or a {{rows}} marker cell")
}

func (service ReportTemplateService) matchersForHeaderRow(rowsData [][]string, headerRowIdx int, colCount int) []*headerMatcher {
	matchers := make([]*headerMatcher, colCount)
	if headerRowIdx < 0 || headerRowIdx >= len(rowsData) {
		return matchers
	}
	header := rowsData[headerRowIdx]
	for cIdx := 0; cIdx < colCount; cIdx++ {
		if cIdx < len(header) {
			matchers[cIdx] = matchHeader(header[cIdx])
		}
	}
	return matchers
}
