package repositories

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"receipt-wrangler/api/internal/commands"
	"receipt-wrangler/api/internal/models"
)

type GroupReceiptSettingsRepository struct {
	BaseRepository
}

func NewGroupReceiptSettingsRepository(tx *gorm.DB) GroupReceiptSettingsRepository {
	repository := GroupReceiptSettingsRepository{BaseRepository: BaseRepository{
		DB: GetDB(),
		TX: tx,
	}}
	return repository
}

func (repository GroupReceiptSettingsRepository) CreateGroupReceiptSettings(groupId uint) (models.GroupReceiptSettings, error) {
	db := repository.GetDB()

	groupReceiptSettingsToCreate := models.GroupReceiptSettings{
		GroupId: groupId,
	}

	err := db.Model(models.GroupReceiptSettings{}).Create(&groupReceiptSettingsToCreate).Error
	if err != nil {
		return models.GroupReceiptSettings{}, err
	}

	return groupReceiptSettingsToCreate, nil
}

// GetGroupReceiptSettings loads the receipt settings for a group, including
// the associated tax rules, for use by reporting/export logic.
func (repository GroupReceiptSettingsRepository) GetGroupReceiptSettings(groupId string) (models.GroupReceiptSettings, error) {
	db := repository.GetDB()

	var groupReceiptSettings models.GroupReceiptSettings
	err := db.Model(&groupReceiptSettings).Where("group_id = ?", groupId).Preload(clause.Associations).First(&groupReceiptSettings).Error
	if err != nil {
		return models.GroupReceiptSettings{}, err
	}

	return groupReceiptSettings, nil
}

// SetReportTemplate records (or clears) the uploaded custom report template
// metadata for a group. Pass empty name/templateType to clear it.
func (repository GroupReceiptSettingsRepository) SetReportTemplate(groupId string, name string, templateType string) (models.GroupReceiptSettings, error) {
	db := repository.GetDB()

	var groupReceiptSettings models.GroupReceiptSettings
	if err := db.Where("group_id = ?", groupId).First(&groupReceiptSettings).Error; err != nil {
		return models.GroupReceiptSettings{}, err
	}

	groupReceiptSettings.ReportTemplateName = name
	groupReceiptSettings.ReportTemplateType = templateType
	if err := db.Model(&groupReceiptSettings).Select("ReportTemplateName", "ReportTemplateType").Updates(&groupReceiptSettings).Error; err != nil {
		return models.GroupReceiptSettings{}, err
	}

	return groupReceiptSettings, nil
}

func (repository GroupReceiptSettingsRepository) UpdateGroupReceiptSettings(
	groupId string,
	command commands.UpdateGroupReceiptSettingsCommand,
) (models.GroupReceiptSettings, error) {
	db := repository.GetDB()

	var groupReceiptSettings models.GroupReceiptSettings

	err := db.Model(&groupReceiptSettings).Where("group_id = ?", groupId).Preload(clause.Associations).First(&groupReceiptSettings).Error
	if err != nil {
		return models.GroupReceiptSettings{}, err
	}

	groupReceiptSettings.HideImages = command.HideImages
	groupReceiptSettings.HideReceiptCategories = command.HideReceiptCategories
	groupReceiptSettings.HideReceiptTags = command.HideReceiptTags
	groupReceiptSettings.HideItemCategories = command.HideItemCategories
	groupReceiptSettings.HideItemTags = command.HideItemTags
	groupReceiptSettings.HideComments = command.HideComments
	groupReceiptSettings.HideShareCategories = command.HideShareCategories
	groupReceiptSettings.HideShareTags = command.HideShareTags
	groupReceiptSettings.HomeCurrency = command.HomeCurrency
	groupReceiptSettings.UsePrintedTax = command.UsePrintedTax
	groupReceiptSettings.DefaultTaxRate = command.DefaultTaxRate
	groupReceiptSettings.TaxRules = command.TaxRules

	err = db.Transaction(func(tx *gorm.DB) error {
		if txErr := tx.Session(&gorm.Session{FullSaveAssociations: false}).Omit("TaxRules").Select("*").Model(&groupReceiptSettings).Updates(&groupReceiptSettings).Error; txErr != nil {
			return txErr
		}

		// True replace: drop existing rules for this settings row, then insert the new set.
		if txErr := tx.Where("group_receipt_settings_id = ?", groupReceiptSettings.ID).Delete(&models.GroupTaxRule{}).Error; txErr != nil {
			return txErr
		}

		for i := range groupReceiptSettings.TaxRules {
			groupReceiptSettings.TaxRules[i].ID = 0
			groupReceiptSettings.TaxRules[i].GroupReceiptSettingsId = groupReceiptSettings.ID
		}
		if len(groupReceiptSettings.TaxRules) > 0 {
			if txErr := tx.Create(&groupReceiptSettings.TaxRules).Error; txErr != nil {
				return txErr
			}
		}

		return nil
	})
	if err != nil {
		return models.GroupReceiptSettings{}, err
	}

	return groupReceiptSettings, nil
}
