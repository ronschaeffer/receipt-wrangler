package models

import "github.com/shopspring/decimal"

// GroupTaxRule expresses how tax should be treated for receipts from a given
// country, for a group's expense reporting. It is a child collection of
// GroupReceiptSettings, allowing the tax rules of one or more jurisdictions to
// be applied (e.g. UK VAT reclaimable with a supplier VAT number, foreign tax
// not reclaimable).
type GroupTaxRule struct {
	BaseModel
	GroupReceiptSettingsId uint `json:"groupReceiptSettingsId"`
	// CountryCode is the ISO 3166-1 alpha-2 country this rule applies to.
	CountryCode string `gorm:"type:varchar(2);not null" json:"countryCode"`
	// Reclaimable indicates whether tax on receipts from this country can be reclaimed.
	Reclaimable bool `gorm:"not null;default:false" json:"reclaimable"`
	// RequireTaxId indicates whether a supplier tax/VAT registration number must
	// be present on the receipt for the tax to be reclaimable.
	RequireTaxId bool `gorm:"not null;default:false" json:"requireTaxId"`
	// DefaultRate is the fallback tax rate (e.g. 0.20 for 20%) used only when no
	// printed tax amount is available and UsePrintedTax allows a computed fallback.
	DefaultRate *decimal.Decimal `gorm:"type:decimal(6,4)" json:"defaultRate,omitempty"`
	// Label is an optional human-readable description of the rule.
	Label string `json:"label"`
}
