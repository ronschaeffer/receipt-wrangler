package services

import (
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"strings"

	"gorm.io/gorm"
	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/repositories"
	"receipt-wrangler/api/internal/utils"
)

// ReceiptPackService assembles a "receipts pack" PDF for a report: one page
// per receipt, each headed with a stable #N reference (matching the export
// CSV line), the receipt's date, vendor, amount, category and VAT, followed by
// the receipt image(s). Missing images produce a clearly flagged placeholder
// page so every claim line is still represented. Rendering reuses the existing
// HtmlToPdfService (headless chromium), and images are embedded as base64
// data: URIs, which that service allows by default.
type ReceiptPackService struct {
	BaseService
}

func NewReceiptPackService(tx *gorm.DB) ReceiptPackService {
	return ReceiptPackService{
		BaseService: BaseService{
			DB: repositories.GetDB(),
			TX: tx,
		},
	}
}

func (service ReceiptPackService) BuildReceiptPack(report models.Report) ([]byte, error) {
	htmlBody := service.buildPackHtml(report)

	htmlToPdfService := NewHtmlToPdfService(service.TX)
	pdfBytes, _, err := htmlToPdfService.Render(htmlBody)
	if err != nil {
		return nil, err
	}

	return pdfBytes, nil
}

func (service ReceiptPackService) buildPackHtml(report models.Report) string {
	fileRepository := repositories.NewFileRepository(service.TX)

	var pages strings.Builder
	for index, receipt := range report.Receipts {
		reference := fmt.Sprintf("#%d", index+1)
		pages.WriteString(service.buildReceiptPage(fileRepository, reference, receipt))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body { font-family: Arial, Helvetica, sans-serif; margin: 0; color: #1a1a1a; }
  .page { page-break-after: always; padding: 24px; box-sizing: border-box; }
  .page:last-child { page-break-after: auto; }
  .header { border-bottom: 2px solid #333; padding-bottom: 8px; margin-bottom: 16px; }
  .ref { font-size: 22px; font-weight: bold; }
  .meta { font-size: 13px; color: #444; margin-top: 4px; line-height: 1.5; }
  .meta strong { color: #1a1a1a; }
  .images { text-align: center; }
  .images img { max-width: 100%%; max-height: 760px; margin: 8px auto; display: block; border: 1px solid #ddd; }
  .placeholder { border: 2px dashed #c0392b; color: #c0392b; padding: 40px; text-align: center; font-size: 16px; margin-top: 24px; }
  .report-title { font-size: 14px; color: #666; }
</style>
</head>
<body>
%s
</body>
</html>`, pages.String())
}

func (service ReceiptPackService) buildReceiptPage(fileRepository repositories.FileRepository, reference string, receipt models.Receipt) string {
	categories := make([]string, 0, len(receipt.Categories))
	for _, category := range receipt.Categories {
		categories = append(categories, category.Name)
	}
	categoryString := strings.Join(categories, ", ")
	if categoryString == "" {
		categoryString = "&mdash;"
	}

	vatString := service.buildVatString(receipt)

	var imagesHtml strings.Builder
	if len(receipt.ImageFiles) == 0 {
		imagesHtml.WriteString(`<div class="placeholder">No receipt image on file for this line &mdash; flagged for review.</div>`)
	} else {
		for _, imageFile := range receipt.ImageFiles {
			dataUri, ok := service.buildImageDataUri(fileRepository, receipt.ID, imageFile)
			if !ok {
				imagesHtml.WriteString(fmt.Sprintf(`<div class="placeholder">Image "%s" could not be read &mdash; flagged for review.</div>`, html.EscapeString(imageFile.Name)))
				continue
			}
			imagesHtml.WriteString(fmt.Sprintf(`<img src="%s" alt="%s">`, dataUri, html.EscapeString(imageFile.Name)))
		}
	}

	return fmt.Sprintf(`<div class="page">
  <div class="header">
    <div class="ref">%s &nbsp; %s</div>
    <div class="meta">
      <strong>Date:</strong> %s &nbsp;|&nbsp;
      <strong>Amount:</strong> %s &nbsp;|&nbsp;
      <strong>Category:</strong> %s &nbsp;|&nbsp;
      <strong>VAT:</strong> %s
    </div>
  </div>
  <div class="images">%s</div>
</div>`,
		html.EscapeString(reference),
		html.EscapeString(receipt.Name),
		receipt.Date.Format("2006-01-02"),
		html.EscapeString(receipt.Amount.String()),
		categoryString,
		vatString,
		imagesHtml.String(),
	)
}

// buildVatString surfaces a VAT custom-field value when one is present on the
// receipt. It looks for a currency-typed custom field whose name contains
// "vat" (case-insensitive), matching the VAT custom field configured on the
// live instance, and falls back to a dash when none is set.
func (service ReceiptPackService) buildVatString(receipt models.Receipt) string {
	for _, value := range receipt.CustomFields {
		name := strings.ToLower(value.CustomField.Name)
		if strings.Contains(name, "vat") && value.CurrencyValue != nil {
			return html.EscapeString(value.CurrencyValue.String())
		}
	}
	return "&mdash;"
}

func (service ReceiptPackService) buildImageDataUri(fileRepository repositories.FileRepository, receiptId uint, imageFile models.FileData) (string, bool) {
	path, err := fileRepository.BuildFilePath(
		utils.UintToString(receiptId),
		utils.UintToString(imageFile.ID),
		imageFile.Name,
	)
	if err != nil {
		return "", false
	}

	bytes, err := utils.ReadFile(path)
	if err != nil || len(bytes) == 0 {
		return "", false
	}

	mimeType := imageFile.FileType
	if mimeType == "" {
		mimeType = http.DetectContentType(bytes)
	}

	encoded := base64.StdEncoding.EncodeToString(bytes)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), true
}
