package models

import "github.com/shopspring/decimal"

type GroupReceiptSettings struct {
	BaseModel
	GroupId               uint `gorm:"not null;unique" json:"groupId"`
	HideImages            bool `gorm:"not null;default:false" json:"hideImages"`
	HideReceiptCategories bool `gorm:"not null;default:false" json:"hideReceiptCategories"`
	HideReceiptTags       bool `gorm:"not null;default:false" json:"hideReceiptTags"`
	HideItemCategories    bool `gorm:"not null;default:false" json:"hideItemCategories"`
	HideItemTags          bool `gorm:"not null;default:false" json:"hideItemTags"`
	HideShareCategories   bool `gorm:"not null;default:false" json:"hideShareCategories"`
	HideShareTags         bool `gorm:"not null;default:false" json:"hideShareTags"`
	HideComments          bool `gorm:"not null;default:false" json:"hideComments"`
	HomeCurrency          string           `gorm:"type:varchar(3);not null;default:'GBP'" json:"homeCurrency"`
	UsePrintedTax         bool             `gorm:"not null;default:true" json:"usePrintedTax"`
	DefaultTaxRate        *decimal.Decimal `gorm:"type:decimal(6,4)" json:"defaultTaxRate,omitempty"`
	TaxRules              []GroupTaxRule   `json:"taxRules"`
	// ReportTemplateName is the original filename of an uploaded custom report
	// template (xlsx or csv); empty means no custom template is configured.
	ReportTemplateName    string           `gorm:"type:varchar(255)" json:"reportTemplateName"`
	// ReportTemplateType is the kind of custom template: "xlsx" or "csv".
	ReportTemplateType    string           `gorm:"type:varchar(8)" json:"reportTemplateType"`
	// VatCustomFieldId designates which CURRENCY-type custom field holds the VAT
	// amount for expense reports; nil means use the receipt's own tax field.
	VatCustomFieldId      *uint            `json:"vatCustomFieldId"`
	// CurrencyCustomFieldId designates which SELECT/TEXT-type custom field holds
	// the receipt currency for expense reports; nil means use the receipt's own
	// currency field.
	CurrencyCustomFieldId *uint            `json:"currencyCustomFieldId"`
}
