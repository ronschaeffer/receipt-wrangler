package repositories

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"receipt-wrangler/api/internal/commands"
	"receipt-wrangler/api/internal/models"
)

type ReportRepository struct {
	BaseRepository
}

func NewReportRepository(tx *gorm.DB) ReportRepository {
	repository := ReportRepository{BaseRepository: BaseRepository{
		DB: GetDB(),
		TX: tx,
	}}
	return repository
}

func (repository ReportRepository) GetReportsByGroupId(groupId string) ([]models.Report, error) {
	db := repository.GetDB()
	var reports []models.Report

	err := db.Table("reports").Where("group_id = ?", groupId).Find(&reports).Error
	if err != nil {
		return nil, err
	}

	return reports, nil
}

// GetReportById loads a single report. When loadReceipts is true the report's
// receipts are eager-loaded with the full association set (items, categories,
// tags, image files, custom fields, paid-by user) so export and pack builders
// have everything they need in one query.
func (repository ReportRepository) GetReportById(reportId uint, loadReceipts bool) (models.Report, error) {
	db := repository.GetDB()
	report := models.Report{}

	query := db.Model(&models.Report{})
	if loadReceipts {
		query = query.
			Preload("Receipts.PaidByUser").
			Preload("Receipts.Categories").
			Preload("Receipts.Tags").
			Preload("Receipts.ImageFiles").
			Preload("Receipts.ReceiptItems").
			Preload("Receipts.CustomFields").
			Preload("Receipts.CustomFields.CustomField").
			Preload("Receipts")
	}

	err := query.Where("id = ?", reportId).First(&report).Error
	if err != nil {
		return models.Report{}, err
	}

	return report, nil
}

func (repository ReportRepository) CreateReport(command commands.UpsertReportCommand) (models.Report, error) {
	db := repository.GetDB()

	status := command.Status
	if len(status) == 0 {
		status = models.REPORT_DRAFT
	}

	report := models.Report{
		Name:    command.Name,
		GroupId: command.GroupId,
		Status:  status,
	}
	repository.stampStatusDates(&report)

	err := db.Create(&report).Error
	if err != nil {
		return models.Report{}, err
	}

	return report, nil
}

// UpdateReport updates the report's name and status. Status transitions stamp
// SubmittedDate / PaidDate the first time the report enters those states, the
// same way ReceiptStatus pairs with ResolvedDate elsewhere in the codebase.
func (repository ReportRepository) UpdateReport(reportId uint, command commands.UpsertReportCommand) (models.Report, error) {
	db := repository.GetDB()
	report := models.Report{}

	err := db.Transaction(func(tx *gorm.DB) error {
		tErr := tx.Where("id = ?", reportId).First(&report).Error
		if tErr != nil {
			return tErr
		}

		report.Name = command.Name
		if len(command.Status) > 0 {
			report.Status = command.Status
		}
		repository.stampStatusDates(&report)

		updates := map[string]interface{}{
			"name":           report.Name,
			"status":         report.Status,
			"submitted_date": report.SubmittedDate,
			"paid_date":      report.PaidDate,
		}

		return tx.Model(&models.Report{}).Where("id = ?", reportId).Updates(updates).Error
	})
	if err != nil {
		return models.Report{}, err
	}

	return report, nil
}

// stampStatusDates sets SubmittedDate/PaidDate when the report first reaches
// SUBMITTED/PAID, and clears them if the report is moved back to an earlier
// state, so the recorded dates always reflect the current status.
func (repository ReportRepository) stampStatusDates(report *models.Report) {
	now := time.Now()

	switch report.Status {
	case models.REPORT_DRAFT:
		report.SubmittedDate = nil
		report.PaidDate = nil
	case models.REPORT_SUBMITTED:
		if report.SubmittedDate == nil {
			report.SubmittedDate = &now
		}
		report.PaidDate = nil
	case models.REPORT_PAID:
		if report.SubmittedDate == nil {
			report.SubmittedDate = &now
		}
		if report.PaidDate == nil {
			report.PaidDate = &now
		}
	}
}

func (repository ReportRepository) AddReceiptsToReport(reportId uint, receiptIds []uint) error {
	db := repository.GetDB()

	if len(receiptIds) == 0 {
		return errors.New("no receipt ids provided")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		report := models.Report{}
		tErr := tx.Where("id = ?", reportId).First(&report).Error
		if tErr != nil {
			return tErr
		}

		receipts := make([]models.Receipt, 0, len(receiptIds))
		tErr = tx.Where("id IN ?", receiptIds).Find(&receipts).Error
		if tErr != nil {
			return tErr
		}

		return tx.Model(&report).Association("Receipts").Append(&receipts)
	})
}

func (repository ReportRepository) RemoveReceiptsFromReport(reportId uint, receiptIds []uint) error {
	db := repository.GetDB()

	if len(receiptIds) == 0 {
		return errors.New("no receipt ids provided")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		report := models.Report{}
		tErr := tx.Where("id = ?", reportId).First(&report).Error
		if tErr != nil {
			return tErr
		}

		receipts := make([]models.Receipt, 0, len(receiptIds))
		tErr = tx.Where("id IN ?", receiptIds).Find(&receipts).Error
		if tErr != nil {
			return tErr
		}

		return tx.Model(&report).Association("Receipts").Delete(&receipts)
	})
}

func (repository ReportRepository) DeleteReport(reportId uint) error {
	db := repository.GetDB()

	return db.Transaction(func(tx *gorm.DB) error {
		report := models.Report{}
		report.ID = reportId
		// Clear the join rows first; receipts themselves are untouched.
		tErr := tx.Model(&report).Association("Receipts").Clear()
		if tErr != nil {
			return tErr
		}

		return tx.Where("id = ?", reportId).Delete(&models.Report{}).Error
	})
}

// GetReportGroupId returns the owning group id for a report, used by handlers
// to enforce group-scoped access before acting on the report.
func (repository ReportRepository) GetReportGroupId(reportId uint) (uint, error) {
	db := repository.GetDB()
	report := models.Report{}

	err := db.Table("reports").Select("id, group_id").Where("id = ?", reportId).First(&report).Error
	if err != nil {
		return 0, err
	}

	return report.GroupId, nil
}
