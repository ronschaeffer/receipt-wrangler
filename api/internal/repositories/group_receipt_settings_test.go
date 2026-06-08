package repositories

import (
	"receipt-wrangler/api/internal/commands"
	"receipt-wrangler/api/internal/models"
	"testing"

	"github.com/shopspring/decimal"
)

func setUpGroupReceiptSettingsTest() {
	CreateTestGroupWithUsers()
	// CreateTestGroupWithUsers seeds group rows directly and does not cascade
	// receipt settings, so create the settings row explicitly for group 1.
	repository := NewGroupReceiptSettingsRepository(nil)
	repository.CreateGroupReceiptSettings(1)
}

func tearDownGroupReceiptSettingsTest() {
	TruncateTestDb()
}

func TestGroupReceiptSettingsDefaultsToGbpHomeCurrency(t *testing.T) {
	defer tearDownGroupReceiptSettingsTest()
	setUpGroupReceiptSettingsTest()

	db := GetDB()
	var settings models.GroupReceiptSettings
	if err := db.Where("group_id = ?", 1).First(&settings).Error; err != nil {
		t.Fatalf("unexpected error loading settings: %v", err)
	}

	if settings.HomeCurrency != "GBP" {
		t.Errorf("expected default home currency GBP, got %q", settings.HomeCurrency)
	}
}

func TestUpdateGroupReceiptSettingsPersistsTaxRules(t *testing.T) {
	defer tearDownGroupReceiptSettingsTest()
	setUpGroupReceiptSettingsTest()

	repository := NewGroupReceiptSettingsRepository(nil)
	rate := decimal.NewFromFloat(0.20)
	command := commands.UpdateGroupReceiptSettingsCommand{
		HomeCurrency:  "EUR",
		UsePrintedTax: true,
		TaxRules: []models.GroupTaxRule{
			{CountryCode: "GB", Reclaimable: true, RequireTaxId: true, DefaultRate: &rate, Label: "UK VAT"},
			{CountryCode: "US", Reclaimable: false, RequireTaxId: false, Label: "US sales tax"},
		},
	}

	updated, err := repository.UpdateGroupReceiptSettings("1", command)
	if err != nil {
		t.Fatalf("unexpected error updating settings: %v", err)
	}

	if updated.HomeCurrency != "EUR" {
		t.Errorf("expected home currency EUR, got %q", updated.HomeCurrency)
	}

	if len(updated.TaxRules) != 2 {
		t.Fatalf("expected 2 tax rules, got %d", len(updated.TaxRules))
	}

	// Reload from the DB to confirm persistence and association linkage.
	db := GetDB()
	var reloaded models.GroupReceiptSettings
	if err := db.Where("group_id = ?", 1).Preload("TaxRules").First(&reloaded).Error; err != nil {
		t.Fatalf("unexpected error reloading settings: %v", err)
	}
	if len(reloaded.TaxRules) != 2 {
		t.Fatalf("expected 2 persisted tax rules, got %d", len(reloaded.TaxRules))
	}

	foundGb := false
	for _, rule := range reloaded.TaxRules {
		if rule.CountryCode == "GB" {
			foundGb = true
			if !rule.Reclaimable || !rule.RequireTaxId {
				t.Errorf("GB rule should be reclaimable and require a tax id; got %+v", rule)
			}
		}
	}
	if !foundGb {
		t.Errorf("expected a GB tax rule to be persisted")
	}
}

func TestUpdateGroupReceiptSettingsReplacesTaxRules(t *testing.T) {
	defer tearDownGroupReceiptSettingsTest()
	setUpGroupReceiptSettingsTest()

	repository := NewGroupReceiptSettingsRepository(nil)

	first := commands.UpdateGroupReceiptSettingsCommand{
		HomeCurrency: "GBP",
		TaxRules: []models.GroupTaxRule{
			{CountryCode: "GB", Reclaimable: true, RequireTaxId: true},
			{CountryCode: "IE", Reclaimable: true, RequireTaxId: true},
		},
	}
	if _, err := repository.UpdateGroupReceiptSettings("1", first); err != nil {
		t.Fatalf("unexpected error on first update: %v", err)
	}

	second := commands.UpdateGroupReceiptSettingsCommand{
		HomeCurrency: "GBP",
		TaxRules: []models.GroupTaxRule{
			{CountryCode: "DE", Reclaimable: true, RequireTaxId: true},
		},
	}
	if _, err := repository.UpdateGroupReceiptSettings("1", second); err != nil {
		t.Fatalf("unexpected error on second update: %v", err)
	}

	db := GetDB()
	var rules []models.GroupTaxRule
	if err := db.Find(&rules).Error; err != nil {
		t.Fatalf("unexpected error loading rules: %v", err)
	}
	if len(rules) != 1 || rules[0].CountryCode != "DE" {
		t.Errorf("expected exactly one DE rule after replace, got %+v", rules)
	}
}
