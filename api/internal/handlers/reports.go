package handlers

import (
	"errors"
	"net/http"
	"receipt-wrangler/api/internal/commands"
	"receipt-wrangler/api/internal/constants"
	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/repositories"
	"receipt-wrangler/api/internal/services"
	"receipt-wrangler/api/internal/structs"
	"receipt-wrangler/api/internal/utils"

	"github.com/go-chi/chi/v5"
)

// GetReportsForGroup lists every report owned by a group.
func GetReportsForGroup(w http.ResponseWriter, r *http.Request) {
	groupId := chi.URLParam(r, "groupId")
	handler := structs.Handler{
		ErrorMessage: "Error retrieving reports",
		Writer:       w,
		Request:      r,
		GroupId:      groupId,
		GroupRole:    models.VIEWER,
		ResponseType: constants.ApplicationJson,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			reportRepository := repositories.NewReportRepository(nil)
			reports, err := reportRepository.GetReportsByGroupId(groupId)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			bytes, err := utils.MarshalResponseData(&reports)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			w.WriteHeader(http.StatusOK)
			w.Write(bytes)
			return 0, nil
		},
	}

	HandleRequest(handler)
}

// GetReport returns a single report with its receipts loaded.
func GetReport(w http.ResponseWriter, r *http.Request) {
	reportId, groupId, err := resolveReportAndGroup(r)
	if err != nil {
		utils.WriteCustomErrorResponse(w, "Report not found", http.StatusNotFound)
		return
	}

	handler := structs.Handler{
		ErrorMessage: "Error retrieving report",
		Writer:       w,
		Request:      r,
		GroupId:      utils.UintToString(groupId),
		GroupRole:    models.VIEWER,
		ResponseType: constants.ApplicationJson,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			reportRepository := repositories.NewReportRepository(nil)
			report, err := reportRepository.GetReportById(reportId, true)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			bytes, err := utils.MarshalResponseData(&report)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			w.WriteHeader(http.StatusOK)
			w.Write(bytes)
			return 0, nil
		},
	}

	HandleRequest(handler)
}

// CreateReport creates a new report in a group. Access is checked against the
// groupId supplied in the body.
func CreateReport(w http.ResponseWriter, r *http.Request) {
	command := commands.UpsertReportCommand{}
	err := command.LoadDataFromRequest(w, r)
	if err != nil {
		utils.WriteCustomErrorResponse(w, "Error creating report", http.StatusInternalServerError)
		return
	}

	handler := structs.Handler{
		ErrorMessage: "Error creating report",
		Writer:       w,
		Request:      r,
		GroupId:      utils.UintToString(command.GroupId),
		GroupRole:    models.EDITOR,
		ResponseType: constants.ApplicationJson,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			vErr := command.Validate()
			if len(vErr.Errors) > 0 {
				structs.WriteValidatorErrorResponse(w, vErr, http.StatusBadRequest)
				return 0, nil
			}

			reportRepository := repositories.NewReportRepository(nil)
			report, err := reportRepository.CreateReport(command)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			bytes, err := utils.MarshalResponseData(&report)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			w.WriteHeader(http.StatusOK)
			w.Write(bytes)
			return 0, nil
		},
	}

	HandleRequest(handler)
}

// UpdateReport updates a report's name and status. Status transitions stamp the
// submitted/paid dates in the repository.
func UpdateReport(w http.ResponseWriter, r *http.Request) {
	reportId, groupId, err := resolveReportAndGroup(r)
	if err != nil {
		utils.WriteCustomErrorResponse(w, "Report not found", http.StatusNotFound)
		return
	}

	command := commands.UpsertReportCommand{}
	err = command.LoadDataFromRequest(w, r)
	if err != nil {
		utils.WriteCustomErrorResponse(w, "Error updating report", http.StatusInternalServerError)
		return
	}
	// The owning group is authoritative from the persisted report, not the body.
	command.GroupId = groupId

	handler := structs.Handler{
		ErrorMessage: "Error updating report",
		Writer:       w,
		Request:      r,
		GroupId:      utils.UintToString(groupId),
		GroupRole:    models.EDITOR,
		ResponseType: constants.ApplicationJson,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			vErr := command.Validate()
			if len(vErr.Errors) > 0 {
				structs.WriteValidatorErrorResponse(w, vErr, http.StatusBadRequest)
				return 0, nil
			}

			reportRepository := repositories.NewReportRepository(nil)
			report, err := reportRepository.UpdateReport(reportId, command)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			bytes, err := utils.MarshalResponseData(&report)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			w.WriteHeader(http.StatusOK)
			w.Write(bytes)
			return 0, nil
		},
	}

	HandleRequest(handler)
}

// DeleteReport removes a report and its join rows. Receipts are untouched.
func DeleteReport(w http.ResponseWriter, r *http.Request) {
	reportId, groupId, err := resolveReportAndGroup(r)
	if err != nil {
		utils.WriteCustomErrorResponse(w, "Report not found", http.StatusNotFound)
		return
	}

	handler := structs.Handler{
		ErrorMessage: "Error deleting report",
		Writer:       w,
		Request:      r,
		GroupId:      utils.UintToString(groupId),
		GroupRole:    models.EDITOR,
		ResponseType: constants.ApplicationJson,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			reportRepository := repositories.NewReportRepository(nil)
			err := reportRepository.DeleteReport(reportId)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			w.WriteHeader(http.StatusOK)
			return 0, nil
		},
	}

	HandleRequest(handler)
}

// AddReceiptsToReport assigns a multi-selected set of receipts to a report,
// mirroring the bulk-status-update gesture.
func AddReceiptsToReport(w http.ResponseWriter, r *http.Request) {
	bulkReportReceiptHandler(w, r, true)
}

// RemoveReceiptsFromReport removes a set of receipts from a report.
func RemoveReceiptsFromReport(w http.ResponseWriter, r *http.Request) {
	bulkReportReceiptHandler(w, r, false)
}

func bulkReportReceiptHandler(w http.ResponseWriter, r *http.Request, add bool) {
	reportId, groupId, err := resolveReportAndGroup(r)
	if err != nil {
		utils.WriteCustomErrorResponse(w, "Report not found", http.StatusNotFound)
		return
	}

	command := commands.BulkReportReceiptCommand{}
	err = command.LoadDataFromRequest(w, r)
	if err != nil {
		utils.WriteCustomErrorResponse(w, "Error updating report receipts", http.StatusInternalServerError)
		return
	}

	errorMessage := "Error adding receipts to report"
	if !add {
		errorMessage = "Error removing receipts from report"
	}

	handler := structs.Handler{
		ErrorMessage: errorMessage,
		Writer:       w,
		Request:      r,
		GroupId:      utils.UintToString(groupId),
		GroupRole:    models.EDITOR,
		ResponseType: constants.ApplicationJson,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			if len(command.ReceiptIds) == 0 {
				return http.StatusBadRequest, errors.New("receiptIds required")
			}

			reportRepository := repositories.NewReportRepository(nil)
			if add {
				err = reportRepository.AddReceiptsToReport(reportId, command.ReceiptIds)
			} else {
				err = reportRepository.RemoveReceiptsFromReport(reportId, command.ReceiptIds)
			}
			if err != nil {
				return http.StatusInternalServerError, err
			}

			report, err := reportRepository.GetReportById(reportId, true)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			bytes, err := utils.MarshalResponseData(&report)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			w.WriteHeader(http.StatusOK)
			w.Write(bytes)
			return 0, nil
		},
	}

	HandleRequest(handler)
}

// ExportReportCsv returns the report's receipts as the standard zipped CSV
// bundle, reusing ReceiptCsvService.
func ExportReportCsv(w http.ResponseWriter, r *http.Request) {
	reportId, groupId, err := resolveReportAndGroup(r)
	if err != nil {
		utils.WriteCustomErrorResponse(w, "Report not found", http.StatusNotFound)
		return
	}

	handler := structs.Handler{
		ErrorMessage: "Error exporting report",
		Writer:       w,
		Request:      r,
		GroupId:      utils.UintToString(groupId),
		GroupRole:    models.VIEWER,
		ResponseType: constants.ApplicationZip,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			reportRepository := repositories.NewReportRepository(nil)
			report, err := reportRepository.GetReportById(reportId, true)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			receiptCsvService := services.NewReceiptCsvService()
			zip, err := receiptCsvService.GetZippedCsvFiles(report.Receipts)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			w.Header().Set("Content-Disposition", "attachment; filename=report.zip")
			w.WriteHeader(http.StatusOK)
			w.Write(zip)
			return 0, nil
		},
	}

	HandleRequest(handler)
}

// ExportReportReceiptPack returns a single PDF containing one page per receipt
// in the report, each headed with a stable #N reference matching the CSV.
func ExportReportReceiptPack(w http.ResponseWriter, r *http.Request) {
	reportId, groupId, err := resolveReportAndGroup(r)
	if err != nil {
		utils.WriteCustomErrorResponse(w, "Report not found", http.StatusNotFound)
		return
	}

	handler := structs.Handler{
		ErrorMessage: "Error building receipt pack",
		Writer:       w,
		Request:      r,
		GroupId:      utils.UintToString(groupId),
		GroupRole:    models.VIEWER,
		ResponseType: constants.ApplicationPdf,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			reportRepository := repositories.NewReportRepository(nil)
			report, err := reportRepository.GetReportById(reportId, true)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			packService := services.NewReceiptPackService(nil)
			pdfBytes, err := packService.BuildReceiptPack(report)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			w.Header().Set("Content-Disposition", "attachment; filename=receipt-pack.pdf")
			w.WriteHeader(http.StatusOK)
			w.Write(pdfBytes)
			return 0, nil
		},
	}

	HandleRequest(handler)
}

// ExportReportXlsx returns the report as a populated expense-report workbook
// (.xlsx), reproducing the standard four-sheet layout and applying the group's
// home currency and tax rules.
func ExportReportXlsx(w http.ResponseWriter, r *http.Request) {
	reportId, groupId, err := resolveReportAndGroup(r)
	if err != nil {
		utils.WriteCustomErrorResponse(w, "Report not found", http.StatusNotFound)
		return
	}

	handler := structs.Handler{
		ErrorMessage: "Error exporting report",
		Writer:       w,
		Request:      r,
		GroupId:      utils.UintToString(groupId),
		GroupRole:    models.VIEWER,
		ResponseType: constants.ApplicationXlsx,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			reportRepository := repositories.NewReportRepository(nil)
			report, err := reportRepository.GetReportById(reportId, true)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			settingsRepository := repositories.NewGroupReceiptSettingsRepository(nil)
			settings, err := settingsRepository.GetGroupReceiptSettings(utils.UintToString(groupId))
			if err != nil {
				return http.StatusInternalServerError, err
			}

			xlsxService := services.NewReportXlsxService(nil)
			fileBytes, err := xlsxService.BuildReportXlsx(report, settings)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			w.Header().Set("Content-Disposition", "attachment; filename=expense-report.xlsx")
			w.WriteHeader(http.StatusOK)
			w.Write(fileBytes)
			return 0, nil
		},
	}

	HandleRequest(handler)
}

// resolveReportAndGroup reads the {reportId} path param and looks up the
// owning group so the handler can enforce group-scoped access against the
// persisted report rather than trusting the request body.
func resolveReportAndGroup(r *http.Request) (uint, uint, error) {
	reportIdParam := chi.URLParam(r, "reportId")
	reportId, err := utils.StringToUint(reportIdParam)
	if err != nil {
		return 0, 0, err
	}

	reportRepository := repositories.NewReportRepository(nil)
	groupId, err := reportRepository.GetReportGroupId(reportId)
	if err != nil {
		return 0, 0, err
	}

	return reportId, groupId, nil
}
