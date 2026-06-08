package services

import (
	"bytes"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"

	"receipt-wrangler/api/internal/models"
)

func strPtr(s string) *string { return &s }
func decPtr(f float64) *decimal.Decimal {
	d := decimal.NewFromFloat(f)
	return &d
}

func buildTestReport() models.Report {
	gbpReceipt := models.Receipt{
		Name:       "Pret a Manger",
		Amount:     decimal.NewFromFloat(7.25),
		Date:       time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		Categories: []models.Category{{Name: "Accommodation & Subsistence"}},
		CountryCode: strPtr("GB"),
		SupplierTaxId: strPtr("GB123456789"),
		TaxAmount:  decPtr(1.21),
	}
	usdReceipt := models.Receipt{
		Name:           "Earl's",
		Amount:         decimal.NewFromFloat(305.39),
		Date:           time.Date(2025, 9, 4, 0, 0, 0, 0, time.UTC),
		Categories:     []models.Category{{Name: "Entertainment Non UK"}},
		Currency:       strPtr("USD"),
		OriginalAmount: decPtr(409.94),
		CountryCode:    strPtr("US"),
		TaxAmount:      decPtr(20.00),
	}
	return models.Report{Receipts: []models.Receipt{gbpReceipt, usdReceipt}}
}

func buildTestSettings() models.GroupReceiptSettings {
	rate := decimal.NewFromFloat(0.20)
	return models.GroupReceiptSettings{
		HomeCurrency:  "GBP",
		UsePrintedTax: true,
		TaxRules: []models.GroupTaxRule{
			{CountryCode: "GB", Reclaimable: true, RequireTaxId: true, DefaultRate: &rate},
			{CountryCode: "US", Reclaimable: false},
		},
	}
}

func TestBuildReportXlsxProducesFourSheets(t *testing.T) {
	service := NewReportXlsxService(nil)
	data, err := service.BuildReportXlsx(buildTestReport(), buildTestSettings())
	if err != nil {
		t.Fatalf("unexpected error building xlsx: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty xlsx output")
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output is not a valid xlsx: %v", err)
	}
	defer f.Close()

	want := []string{"All Expenses", "Summary by Category", "Summary by Trip", "Foreign Currency"}
	sheets := f.GetSheetList()
	have := map[string]bool{}
	for _, s := range sheets {
		have[s] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("expected sheet %q to be present; got %v", w, sheets)
		}
	}
}

func TestBuildReportXlsxHeadersAndTotals(t *testing.T) {
	service := NewReportXlsxService(nil)
	data, err := service.BuildReportXlsx(buildTestReport(), buildTestSettings())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	defer f.Close()

	// All Expenses headers reproduce the reference layout, with a Review column added.
	expectHeaders := map[string]string{
		"A4": "#", "B4": "Date", "C4": "Merchant", "D4": "Category",
		"E4": "Receipt", "F4": "Orig. Amt", "G4": "Cur", "J4": "Review",
	}
	for cell, want := range expectHeaders {
		got, _ := f.GetCellValue("All Expenses", cell)
		if got != want {
			t.Errorf("All Expenses %s = %q, want %q", cell, got, want)
		}
	}

	// Totals row uses SUM formulas (2 data rows => row 7).
	formula, _ := f.GetCellFormula("All Expenses", "I7")
	if formula == "" {
		t.Errorf("expected a SUM formula in the Total column totals cell")
	}

	// Foreign Currency sheet should contain the USD row with an implied-rate formula.
	rate, _ := f.GetCellFormula("Foreign Currency", "F5")
	if rate == "" {
		t.Errorf("expected implied-rate formula on Foreign Currency sheet")
	}
}

func TestBuildReportXlsxFlagsAndReclaim(t *testing.T) {
	service := NewReportXlsxService(nil)
	data, err := service.BuildReportXlsx(buildTestReport(), buildTestSettings())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	defer f.Close()

	// The USD row (row 6, second receipt) is foreign => Review should be populated.
	usdReview, _ := f.GetCellValue("All Expenses", "J6")
	if usdReview == "" {
		t.Errorf("expected foreign USD row to carry a review note")
	}

	// Reclaimable VAT total should equal the GB receipt's tax (1.21), since US is not reclaimable.
	// It is written two rows below the category TOTAL row; search for the label.
	found := false
	for r := 1; r <= 30; r++ {
		label, _ := f.GetCellValue("Summary by Category", "A"+itoa(r))
		if label == "Reclaimable VAT" {
			val, _ := f.GetCellValue("Summary by Category", "C"+itoa(r))
			if val != "1.21" {
				t.Errorf("reclaimable VAT = %q, want 1.21", val)
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a Reclaimable VAT total on Summary by Category")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
