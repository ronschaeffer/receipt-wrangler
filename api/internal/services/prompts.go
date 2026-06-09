package services

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"receipt-wrangler/api/internal/commands"
	"receipt-wrangler/api/internal/constants"
	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/repositories"
)

type PromptService struct {
	BaseService
}

func NewPromptService(tx *gorm.DB) PromptService {
	service := PromptService{BaseService: BaseService{
		DB: repositories.GetDB(),
		TX: tx,
	}}
	return service
}

func (service PromptService) CreateDefaultPrompt() (models.Prompt, error) {
	db := service.GetDB()
	var defaultPromptCount int64
	db.Model(models.Prompt{}).Where("name = ?", constants.DefaultPromptName).Count(&defaultPromptCount)

	defaultPrompt := fmt.Sprintf(`
Find the receipt's name, total cost, currency, country, tax, and date. Format the found data as:
{
	"name": store name,
	"amount": amount as a number,
	"currency": ISO 4217 currency code printed on the receipt,
	"originalAmount": amount as printed on the receipt, before any conversion,
	"taxAmount": tax/VAT amount as printed on the receipt, as a number,
	"supplierTaxId": supplier VAT/GST/tax registration number if shown,
	"countryCode": ISO 3166-1 alpha-2 country code of the merchant, from the receipt's address or locale,
	"date": date in ISO 8601 format in UTC with ALL time values set as 0,
	"categories": categories,
	"tags": tags
}
If a store name cannot be confidently found, use 'Default store name' as the default name.
Omit any value if not found with confidence. Assume the date is in the year @currentYear if not provided.
The amount must be a float or integer.
For "currency": read it from the receipt (symbols, codes, tax labels, address/locale). Do NOT assume the currency is the home currency just because a local symbol appears. If the currency cannot be determined, omit the field (do not guess).
For "amount": if the receipt shows a higher total actually charged (e.g. a tip was added), use that charged total, not the pre-tip subtotal.
For "taxAmount" and "supplierTaxId": only include them if they are explicitly printed on the receipt; never infer or compute a tax amount that is not shown. Omit if absent.
For "countryCode": determine it from the merchant address or locale on the receipt; omit if it cannot be determined.
If the receipt represents a refund, return, or credit (money returned to the customer rather than charged), the amount and item amounts MUST be negative. Otherwise they are positive.

Please do NOT add any additional information, only valid JSON.
Please return the json in plaintext ONLY, do not ever return it in a code block or any other format.

Choose at most ONE category from the given list based on the receipt's items and store name. Return a single category in the array (or an empty array if none fit). If no categories fit, please return an empty array for the field and do not select any categories. When selecting categories, select only the id, like:
{
	Id: category id
}

Emphasize the relationship between the category and the receipt, and use the description of the category to fine tune the results. Do not return categories that have an empty name or do not exist.
If there are no categories to chose from, then please make categories an empty array.
Likewise, if there are not tags to choose from, then make tags an empty array.

Categories to chose from: @categories

Follow the same process as described for categories for tags.

Tags to chose from: @tags

Receipt text: @ocrText
`)

	if defaultPromptCount > 0 {
		return models.Prompt{}, errors.New("default prompt already exists")
	}

	promptRepository := repositories.NewPromptRepository(service.TX)
	command := commands.UpsertPromptCommand{
		Name:        constants.DefaultPromptName,
		Description: "Default prompt used for previous versions of Receipt Wrangler.",
		Prompt:      defaultPrompt,
	}

	return promptRepository.CreatePrompt(command)
}
