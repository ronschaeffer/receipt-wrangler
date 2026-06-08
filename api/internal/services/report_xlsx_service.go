package services

import (
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/repositories"
)

// ReportXlsxService builds a populated expense-report workbook from a report's
// receipts, reproducing the four-sheet layout (All Expenses, Summary by
// Category, Summary by Trip, Foreign Currency) and extending it with a Review
// column and a reclaimable-VAT total. Currency and tax treatment are driven by
// the owning group's receipt settings (home currency + per-country tax rules).
type ReportXlsxService struct {
	BaseService
}

func NewReportXlsxService(tx *gorm.DB) ReportXlsxService {
	return ReportXlsxService{BaseService: BaseService{
		DB: repositories.GetDB(),
		TX: tx,
	}}
}

// tripGapDays controls trip grouping: a receipt more than this many days after
// the previous one starts a new trip. Heuristic, mirrors the worked example.
const tripGapDays = 2

type xlsxRow struct {
	index       int
	date        time.Time
	merchant    string
	category    string
	hasReceipt  bool
	origAmount  *decimal.Decimal
	currency    string
	tax         decimal.Decimal
	total       decimal.Decimal
	isForeign   bool
	reclaimable bool
	review      string
}

// BuildReportXlsx renders the report to an .xlsx byte slice.
func (service ReportXlsxService) BuildReportXlsx(report models.Report, settings models.GroupReceiptSettings) ([]byte, error) {
	homeCurrency := settings.HomeCurrency
	if homeCurrency == "" {
		homeCurrency = "GBP"
	}

	rows := service.buildRows(report, settings, homeCurrency)

	f := excelize.NewFile()
	defer f.Close()

	// excelize creates a default "Sheet1"; we rename it to the first sheet.
	if err := service.writeAllExpenses(f, rows, homeCurrency); err != nil {
		return nil, err
	}
	if err := service.writeSummaryByCategory(f, rows, homeCurrency); err != nil {
		return nil, err
	}
	if err := service.writeSummaryByTrip(f, rows, homeCurrency); err != nil {
		return nil, err
	}
	if err := service.writeForeignCurrency(f, rows, homeCurrency); err != nil {
		return nil, err
	}

	// Ensure the default empty sheet is gone and the first real sheet is active.
	if idx, err := f.GetSheetIndex("All Expenses"); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// buildRows maps receipts into report rows, applying currency and tax treatment.
func (service ReportXlsxService) buildRows(report models.Report, settings models.GroupReceiptSettings, homeCurrency string) []xlsxRow {
	receipts := make([]models.Receipt, len(report.Receipts))
	copy(receipts, report.Receipts)
	sort.SliceStable(receipts, func(i, j int) bool {
		return receipts[i].Date.Before(receipts[j].Date)
	})

	ruleByCountry := make(map[string]models.GroupTaxRule)
	for _, rule := range settings.TaxRules {
		ruleByCountry[rule.CountryCode] = rule
	}

	rows := make([]xlsxRow, 0, len(receipts))
	for i, receipt := range receipts {
		row := xlsxRow{
			index:      i + 1,
			date:       receipt.Date,
			merchant:   receipt.Name,
			category:   firstCategoryName(receipt.Categories),
			hasReceipt: len(receipt.ImageFiles) > 0,
			total:      receipt.Amount,
		}

		currency := homeCurrency
		if receipt.Currency != nil && *receipt.Currency != "" {
			currency = *receipt.Currency
		}
		row.currency = currency
		row.isForeign = currency != homeCurrency
		row.origAmount = receipt.OriginalAmount

		if receipt.TaxAmount != nil {
			row.tax = *receipt.TaxAmount
		}

		// Reclaim + review logic.
		var reviewNotes []string
		if row.isForeign {
			reviewNotes = append(reviewNotes, "Foreign currency — verify conversion")
		}
		if receipt.Currency == nil || *receipt.Currency == "" {
			if row.isForeign {
				reviewNotes = append(reviewNotes, "Currency not detected")
			}
		}
		if receipt.Amount.IsZero() {
			reviewNotes = append(reviewNotes, "Zero amount — recover from receipt")
		}
		if !row.hasReceipt {
			reviewNotes = append(reviewNotes, "No receipt image")
		}

		country := ""
		if receipt.CountryCode != nil {
			country = *receipt.CountryCode
		}
		if country == "" {
			if !row.tax.IsZero() {
				reviewNotes = append(reviewNotes, "Country undetermined — reclaim not applied")
			}
		} else if rule, ok := ruleByCountry[country]; ok && rule.Reclaimable {
			if rule.RequireTaxId && (receipt.SupplierTaxId == nil || *receipt.SupplierTaxId == "") {
				reviewNotes = append(reviewNotes, "VAT reclaim needs supplier tax id")
			} else if !row.tax.IsZero() {
				row.reclaimable = true
			}
		}

		row.review = joinNotes(reviewNotes)
		rows = append(rows, row)
	}
	return rows
}

func (service ReportXlsxService) writeAllExpenses(f *excelize.File, rows []xlsxRow, home string) error {
	sheet := "All Expenses"
	f.SetSheetName("Sheet1", sheet)

	f.SetCellValue(sheet, "A1", "All Expenses")
	dateRange := rowsDateRange(rows)
	f.SetCellValue(sheet, "A2", fmt.Sprintf("%d expenses · %s · one row per receipt · foreign-currency rows highlighted", len(rows), dateRange))

	headers := []string{"#", "Date", "Merchant", "Category", "Receipt", "Orig. Amt", "Cur", fmt.Sprintf("Tax (%s)", currencySymbol(home)), fmt.Sprintf("Total (%s)", currencySymbol(home)), "Review"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 4)
		f.SetCellValue(sheet, cell, h)
	}

	foreignStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"FFF2CC"}, Pattern: 1},
	})

	r := 5
	for _, row := range rows {
		f.SetCellValue(sheet, cellAt(1, r), row.index)
		f.SetCellValue(sheet, cellAt(2, r), row.date.Format("2006-01-02"))
		f.SetCellValue(sheet, cellAt(3, r), row.merchant)
		f.SetCellValue(sheet, cellAt(4, r), row.category)
		if row.hasReceipt {
			f.SetCellValue(sheet, cellAt(5, r), "Yes")
		} else {
			f.SetCellValue(sheet, cellAt(5, r), "No")
		}
		if row.origAmount != nil {
			val, _ := row.origAmount.Float64()
			f.SetCellValue(sheet, cellAt(6, r), val)
		}
		if row.isForeign {
			f.SetCellValue(sheet, cellAt(7, r), row.currency)
		}
		taxVal, _ := row.tax.Float64()
		f.SetCellValue(sheet, cellAt(8, r), taxVal)
		totalVal, _ := row.total.Float64()
		f.SetCellValue(sheet, cellAt(9, r), totalVal)
		if row.review != "" {
			f.SetCellValue(sheet, cellAt(10, r), row.review)
		}
		if row.isForeign {
			f.SetCellStyle(sheet, cellAt(1, r), cellAt(10, r), foreignStyle)
		}
		r++
	}

	// Totals row with SUM formulas over Tax and Total columns.
	f.SetCellFormula(sheet, cellAt(8, r), fmt.Sprintf("=SUM(H5:H%d)", r-1))
	f.SetCellFormula(sheet, cellAt(9, r), fmt.Sprintf("=SUM(I5:I%d)", r-1))

	return nil
}

func (service ReportXlsxService) writeSummaryByCategory(f *excelize.File, rows []xlsxRow, home string) error {
	sheet := "Summary by Category"
	f.NewSheet(sheet)

	f.SetCellValue(sheet, "A1", "Summary by Category")
	headers := []string{"Category", "Receipts", fmt.Sprintf("Tax (%s)", currencySymbol(home)), fmt.Sprintf("Total (%s)", currencySymbol(home))}
	for i, h := range headers {
		f.SetCellValue(sheet, cellAt(i+1, 3), h)
	}

	type agg struct {
		count int
		tax   decimal.Decimal
		total decimal.Decimal
	}
	order := []string{}
	byCat := map[string]*agg{}
	var reclaimable decimal.Decimal
	for _, row := range rows {
		cat := row.category
		if cat == "" {
			cat = "Uncategorised"
		}
		if _, ok := byCat[cat]; !ok {
			byCat[cat] = &agg{}
			order = append(order, cat)
		}
		a := byCat[cat]
		a.count++
		a.tax = a.tax.Add(row.tax)
		a.total = a.total.Add(row.total)
		if row.reclaimable {
			reclaimable = reclaimable.Add(row.tax)
		}
	}
	// Sort categories by total descending, mirroring the example ordering.
	sort.SliceStable(order, func(i, j int) bool {
		return byCat[order[i]].total.GreaterThan(byCat[order[j]].total)
	})

	r := 4
	for _, cat := range order {
		a := byCat[cat]
		f.SetCellValue(sheet, cellAt(1, r), cat)
		f.SetCellValue(sheet, cellAt(2, r), a.count)
		taxVal, _ := a.tax.Float64()
		f.SetCellValue(sheet, cellAt(3, r), taxVal)
		totalVal, _ := a.total.Float64()
		f.SetCellValue(sheet, cellAt(4, r), totalVal)
		r++
	}
	f.SetCellValue(sheet, cellAt(1, r), "TOTAL")
	f.SetCellFormula(sheet, cellAt(2, r), fmt.Sprintf("=SUM(B4:B%d)", r-1))
	f.SetCellFormula(sheet, cellAt(3, r), fmt.Sprintf("=SUM(C4:C%d)", r-1))
	f.SetCellFormula(sheet, cellAt(4, r), fmt.Sprintf("=SUM(D4:D%d)", r-1))

	// Extension: reclaimable VAT total.
	r += 2
	f.SetCellValue(sheet, cellAt(1, r), "Reclaimable VAT")
	reclVal, _ := reclaimable.Float64()
	f.SetCellValue(sheet, cellAt(3, r), reclVal)

	return nil
}

func (service ReportXlsxService) writeSummaryByTrip(f *excelize.File, rows []xlsxRow, home string) error {
	sheet := "Summary by Trip"
	f.NewSheet(sheet)

	trips := groupTrips(rows)

	f.SetCellValue(sheet, "A1", "Expense Report — Summary by Trip")
	f.SetCellValue(sheet, "A2", fmt.Sprintf("%s · %d receipts · %d trips", rowsDateRange(rows), len(rows), len(trips)))

	headers := []string{"Trip / Date Range", "Start", "End", "Days", "Receipts", "Foreign", fmt.Sprintf("Tax (%s)", currencySymbol(home)), fmt.Sprintf("Total (%s)", currencySymbol(home))}
	for i, h := range headers {
		f.SetCellValue(sheet, cellAt(i+1, 4), h)
	}

	r := 5
	for _, t := range trips {
		f.SetCellValue(sheet, cellAt(1, r), tripLabel(t.start, t.end))
		f.SetCellValue(sheet, cellAt(2, r), t.start.Format("2006-01-02"))
		f.SetCellValue(sheet, cellAt(3, r), t.end.Format("2006-01-02"))
		f.SetCellValue(sheet, cellAt(4, r), t.days())
		f.SetCellValue(sheet, cellAt(5, r), t.receipts)
		f.SetCellValue(sheet, cellAt(6, r), t.foreign)
		taxVal, _ := t.tax.Float64()
		f.SetCellValue(sheet, cellAt(7, r), taxVal)
		totalVal, _ := t.total.Float64()
		f.SetCellValue(sheet, cellAt(8, r), totalVal)
		r++
	}
	f.SetCellValue(sheet, cellAt(1, r), "TOTAL")
	f.SetCellFormula(sheet, cellAt(5, r), fmt.Sprintf("=SUM(E5:E%d)", r-1))
	f.SetCellFormula(sheet, cellAt(6, r), fmt.Sprintf("=SUM(F5:F%d)", r-1))
	f.SetCellFormula(sheet, cellAt(7, r), fmt.Sprintf("=SUM(G5:G%d)", r-1))
	f.SetCellFormula(sheet, cellAt(8, r), fmt.Sprintf("=SUM(H5:H%d)", r-1))

	return nil
}

func (service ReportXlsxService) writeForeignCurrency(f *excelize.File, rows []xlsxRow, home string) error {
	sheet := "Foreign Currency"
	f.NewSheet(sheet)

	f.SetCellValue(sheet, "A1", "Foreign-Currency Expenses")
	f.SetCellValue(sheet, "A2", fmt.Sprintf("Entries charged in a currency other than %s, with the applied rate. Verify against card statement.", home))

	headers := []string{"Date", "Merchant", "Orig. Amount", "Currency", fmt.Sprintf("%s Total", home), "Implied Rate"}
	for i, h := range headers {
		f.SetCellValue(sheet, cellAt(i+1, 4), h)
	}

	r := 5
	for _, row := range rows {
		if !row.isForeign {
			continue
		}
		f.SetCellValue(sheet, cellAt(1, r), row.date.Format("2006-01-02"))
		f.SetCellValue(sheet, cellAt(2, r), row.merchant)
		if row.origAmount != nil {
			val, _ := row.origAmount.Float64()
			f.SetCellValue(sheet, cellAt(3, r), val)
		}
		f.SetCellValue(sheet, cellAt(4, r), row.currency)
		totalVal, _ := row.total.Float64()
		f.SetCellValue(sheet, cellAt(5, r), totalVal)
		f.SetCellFormula(sheet, cellAt(6, r), fmt.Sprintf("=E%d/C%d", r, r))
		r++
	}

	return nil
}

// --- trip grouping ---

type trip struct {
	start    time.Time
	end      time.Time
	receipts int
	foreign  int
	tax      decimal.Decimal
	total    decimal.Decimal
}

func (t trip) days() int {
	return int(t.end.Sub(t.start).Hours()/24) + 1
}

func groupTrips(rows []xlsxRow) []trip {
	var trips []trip
	var current *trip
	var lastDate time.Time
	for _, row := range rows {
		if current == nil || row.date.Sub(lastDate) > time.Duration(tripGapDays)*24*time.Hour {
			trips = append(trips, trip{start: row.date, end: row.date})
			current = &trips[len(trips)-1]
		}
		current.end = row.date
		current.receipts++
		if row.isForeign {
			current.foreign++
		}
		current.tax = current.tax.Add(row.tax)
		current.total = current.total.Add(row.total)
		lastDate = row.date
	}
	return trips
}

// --- helpers ---

func cellAt(col, row int) string {
	cell, _ := excelize.CoordinatesToCellName(col, row)
	return cell
}

func firstCategoryName(categories []models.Category) string {
	if len(categories) == 0 {
		return ""
	}
	return categories[0].Name
}

func joinNotes(notes []string) string {
	out := ""
	for i, n := range notes {
		if i > 0 {
			out += "; "
		}
		out += n
	}
	return out
}

func rowsDateRange(rows []xlsxRow) string {
	if len(rows) == 0 {
		return ""
	}
	return fmt.Sprintf("%s to %s", rows[0].date.Format("2006-01-02"), rows[len(rows)-1].date.Format("2006-01-02"))
}

func tripLabel(start, end time.Time) string {
	if start.Equal(end) {
		return start.Format("02 Jan 2006")
	}
	if start.Month() == end.Month() && start.Year() == end.Year() {
		return fmt.Sprintf("%s–%s", start.Format("02"), end.Format("02 Jan 2006"))
	}
	return fmt.Sprintf("%s – %s", start.Format("02 Jan"), end.Format("02 Jan 2006"))
}

func currencySymbol(code string) string {
	switch code {
	case "GBP":
		return "£"
	case "USD":
		return "$"
	case "EUR":
		return "€"
	default:
		return code
	}
}
