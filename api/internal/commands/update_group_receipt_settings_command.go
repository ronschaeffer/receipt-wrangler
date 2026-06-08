package commands

import (
	"encoding/json"
	"net/http"
	"github.com/shopspring/decimal"
	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/utils"
)

type UpdateGroupReceiptSettingsCommand struct {
	HideImages            bool `json:"hideImages"`
	HideReceiptCategories bool `json:"hideReceiptCategories"`
	HideReceiptTags       bool `json:"hideReceiptTags"`
	HideItemCategories    bool `json:"hideItemCategories"`
	HideItemTags          bool `json:"hideItemTags"`
	HideComments          bool `json:"hideComments"`
	HideShareCategories   bool `json:"hideShareCategories"`
	HideShareTags         bool `json:"hideShareTags"`
	HomeCurrency          string                 `json:"homeCurrency"`
	UsePrintedTax         bool                   `json:"usePrintedTax"`
	DefaultTaxRate        *decimal.Decimal       `json:"defaultTaxRate,omitempty"`
	TaxRules              []models.GroupTaxRule  `json:"taxRules"`
}

func (command *UpdateGroupReceiptSettingsCommand) LoadDataFromRequest(w http.ResponseWriter, r *http.Request) error {
	bytes, err := utils.GetBodyData(w, r)
	if err != nil {
		return err
	}

	err = json.Unmarshal(bytes, &command)
	if err != nil {
		return err
	}

	return nil
}
