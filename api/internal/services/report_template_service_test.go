package services

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

)

func writeHeaderTemplate(t *testing.T, headers []string, marker bool) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tmpl.xlsx")
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	// header on row 2, leaving a title row above to test detection offset
	f.SetCellValue(sheet, "A1", "My Company Expense Claim")
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet, cell, h)
	}
	if marker {
		f.SetCellValue(sheet, "A3", "{{rows}}")
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save template: %v", err)
	}
	f.Close()
	return path
}

func TestBuildFromTemplateXlsxHeaderMatch(t *testing.T) {
	path := writeHeaderTemplate(t, []string{"Date", "Supplier", "Category", "VAT", "Total"}, false)
	settings := buildTestSettings()
	settings.ReportTemplateType = "xlsx"

	service := NewReportTemplateService(nil)
	data, kind, err := service.BuildFromTemplate(buildTestReport(), settings, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != "xlsx" {
		t.Errorf("kind = %q, want xlsx", kind)
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output not valid xlsx: %v", err)
	}
	defer f.Close()
	sheet := f.GetSheetName(0)

	// Header row was row 2, so data should start row 3. First receipt is the GBP Pret one.
	supplier, _ := f.GetCellValue(sheet, "B3")
	if supplier != "Pret a Manger" {
		t.Errorf("B3 supplier = %q, want Pret a Manger", supplier)
	}
	// "Supplier" header should have mapped to merchant via keyword match.
	date, _ := f.GetCellValue(sheet, "A3")
	if date != "2024-03-01" {
		t.Errorf("A3 date = %q, want 2024-03-01", date)
	}
}

func TestBuildFromTemplateXlsxMarker(t *testing.T) {
	path := writeHeaderTemplate(t, []string{"Date", "Merchant", "Category", "Tax", "Amount"}, true)
	settings := buildTestSettings()
	settings.ReportTemplateType = "xlsx"

	service := NewReportTemplateService(nil)
	data, _, err := service.BuildFromTemplate(buildTestReport(), settings, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	defer f.Close()
	sheet := f.GetSheetName(0)

	// Marker was at A3, so data begins row 3.
	merchant, _ := f.GetCellValue(sheet, "B3")
	if merchant != "Pret a Manger" {
		t.Errorf("B3 = %q, want Pret a Manger", merchant)
	}
	// Marker cell should be cleared (not literally {{rows}} anymore).
	if strings.Contains(merchant, "{{rows}}") {
		t.Errorf("marker not cleared")
	}
}

func TestBuildFromTemplateCsv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmpl.csv")
	if err := os.WriteFile(path, []byte("Date,Vendor,Category,Tax,Total\n"), 0644); err != nil {
		t.Fatal(err)
	}
	settings := buildTestSettings()
	settings.ReportTemplateType = "csv"

	service := NewReportTemplateService(nil)
	data, kind, err := service.BuildFromTemplate(buildTestReport(), settings, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != "csv" {
		t.Errorf("kind = %q, want csv", kind)
	}

	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatalf("output not valid csv: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected header + 2 data rows, got %d", len(records))
	}
	if records[0][1] != "Vendor" {
		t.Errorf("header preserved? got %v", records[0])
	}
	if records[1][1] != "Pret a Manger" {
		t.Errorf("first data row vendor = %q, want Pret a Manger", records[1][1])
	}
}

