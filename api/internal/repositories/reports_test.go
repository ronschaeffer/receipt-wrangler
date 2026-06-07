package repositories

import (
	"receipt-wrangler/api/internal/commands"
	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/utils"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func setupReportTest() {
	CreateTestGroupWithUsers()
}

func tearDownReportTest() {
	TruncateTestDb()
}

func seedReportReceipts(count int) []uint {
	db := GetDB()
	ids := make([]uint, 0, count)
	for i := 0; i < count; i++ {
		receipt := models.Receipt{
			Name:         "receipt",
			Amount:       decimal.NewFromFloat(10.00),
			Date:         time.Now(),
			PaidByUserID: 1,
			Status:       models.OPEN,
			GroupId:      1,
		}
		db.Create(&receipt)
		ids = append(ids, receipt.ID)
	}
	return ids
}

func TestCreateReportDefaultsToDraftWithNoDates(t *testing.T) {
	defer tearDownReportTest()
	setupReportTest()

	repository := NewReportRepository(nil)
	report, err := repository.CreateReport(commands.UpsertReportCommand{
		Name:    "March",
		GroupId: 1,
	})
	if err != nil {
		utils.PrintTestError(t, err, nil)
		return
	}

	if report.Status != models.REPORT_DRAFT {
		utils.PrintTestError(t, report.Status, models.REPORT_DRAFT)
	}
	if report.SubmittedDate != nil || report.PaidDate != nil {
		utils.PrintTestError(t, "dates set on draft", "nil dates")
	}
}

func TestUpdateReportStampsSubmittedAndPaidDates(t *testing.T) {
	defer tearDownReportTest()
	setupReportTest()

	repository := NewReportRepository(nil)
	report, _ := repository.CreateReport(commands.UpsertReportCommand{Name: "March", GroupId: 1})

	submitted, err := repository.UpdateReport(report.ID, commands.UpsertReportCommand{
		Name:    "March",
		GroupId: 1,
		Status:  models.REPORT_SUBMITTED,
	})
	if err != nil {
		utils.PrintTestError(t, err, nil)
		return
	}
	if submitted.SubmittedDate == nil {
		utils.PrintTestError(t, "nil submittedDate", "a submittedDate")
	}
	if submitted.PaidDate != nil {
		utils.PrintTestError(t, "paidDate set on submitted", "nil paidDate")
	}

	paid, _ := repository.UpdateReport(report.ID, commands.UpsertReportCommand{
		Name:    "March",
		GroupId: 1,
		Status:  models.REPORT_PAID,
	})
	if paid.SubmittedDate == nil {
		utils.PrintTestError(t, "submittedDate cleared on paid", "a submittedDate")
	}
	if paid.PaidDate == nil {
		utils.PrintTestError(t, "nil paidDate", "a paidDate")
	}

	// Moving back to draft clears both dates.
	draft, _ := repository.UpdateReport(report.ID, commands.UpsertReportCommand{
		Name:    "March",
		GroupId: 1,
		Status:  models.REPORT_DRAFT,
	})
	if draft.SubmittedDate != nil || draft.PaidDate != nil {
		utils.PrintTestError(t, "dates retained on draft", "nil dates")
	}
}

func TestAddAndRemoveReceiptsToReport(t *testing.T) {
	defer tearDownReportTest()
	setupReportTest()
	receiptIds := seedReportReceipts(3)

	repository := NewReportRepository(nil)
	report, _ := repository.CreateReport(commands.UpsertReportCommand{Name: "March", GroupId: 1})

	err := repository.AddReceiptsToReport(report.ID, receiptIds)
	if err != nil {
		utils.PrintTestError(t, err, nil)
		return
	}

	loaded, _ := repository.GetReportById(report.ID, true)
	if len(loaded.Receipts) != 3 {
		utils.PrintTestError(t, len(loaded.Receipts), 3)
	}

	err = repository.RemoveReceiptsFromReport(report.ID, []uint{receiptIds[0]})
	if err != nil {
		utils.PrintTestError(t, err, nil)
		return
	}

	loaded, _ = repository.GetReportById(report.ID, true)
	if len(loaded.Receipts) != 2 {
		utils.PrintTestError(t, len(loaded.Receipts), 2)
	}
}

func TestDeleteReportPreservesReceipts(t *testing.T) {
	defer tearDownReportTest()
	setupReportTest()
	receiptIds := seedReportReceipts(2)

	repository := NewReportRepository(nil)
	report, _ := repository.CreateReport(commands.UpsertReportCommand{Name: "March", GroupId: 1})
	repository.AddReceiptsToReport(report.ID, receiptIds)

	err := repository.DeleteReport(report.ID)
	if err != nil {
		utils.PrintTestError(t, err, nil)
		return
	}

	// The report is gone.
	_, err = repository.GetReportById(report.ID, false)
	if err == nil {
		utils.PrintTestError(t, "report still retrievable after delete", "not found error")
	}

	// The receipts are untouched.
	db := GetDB()
	var receiptCount int64
	db.Model(&models.Receipt{}).Where("id IN ?", receiptIds).Count(&receiptCount)
	if receiptCount != 2 {
		utils.PrintTestError(t, receiptCount, 2)
	}

	// The join rows are gone.
	var joinCount int64
	db.Table("report_receipts").Where("report_id = ?", report.ID).Count(&joinCount)
	if joinCount != 0 {
		utils.PrintTestError(t, joinCount, 0)
	}
}
