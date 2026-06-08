package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"receipt-wrangler/api/internal/constants"
	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/repositories"
	"receipt-wrangler/api/internal/services"
	"receipt-wrangler/api/internal/structs"
	"receipt-wrangler/api/internal/utils"
)

// UploadReportTemplate stores a per-group custom expense-report template
// (.xlsx or .csv) on disk and records its metadata on the group's receipt
// settings. It supersedes nothing: the default workbook export remains
// available; this adds an optional custom export.
func UploadReportTemplate(w http.ResponseWriter, r *http.Request) {
	groupId := chi.URLParam(r, "groupId")

	handler := structs.Handler{
		ErrorMessage: "Error uploading report template",
		Writer:       w,
		Request:      r,
		GroupId:      groupId,
		GroupRole:    models.OWNER,
		ResponseType: constants.ApplicationJson,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			if err := r.ParseMultipartForm(constants.MultipartFormMaxSize); err != nil {
				return http.StatusInternalServerError, err
			}
			file, fileHeader, err := r.FormFile("file")
			if err != nil {
				return http.StatusBadRequest, err
			}
			defer file.Close()

			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileHeader.Filename), "."))
			if ext != "xlsx" && ext != "csv" {
				utils.WriteCustomErrorResponse(w, "Template must be a .xlsx or .csv file", http.StatusBadRequest)
				return 0, errors.New("invalid template extension")
			}

			fileBytes := make([]byte, fileHeader.Size)
			if _, err := file.Read(fileBytes); err != nil {
				return http.StatusInternalServerError, err
			}

			group, err := getGroupForTemplate(groupId)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			templatePath, err := utils.BuildReportTemplatePath(groupId, group.Name, ext)
			if err != nil {
				return http.StatusInternalServerError, err
			}
			if err := os.MkdirAll(filepath.Dir(templatePath), 0755); err != nil {
				return http.StatusInternalServerError, err
			}
			// Remove any previously stored template of the other type so only one is active.
			removeOtherTemplate(groupId, group.Name, ext)
			if err := os.WriteFile(templatePath, fileBytes, 0644); err != nil {
				return http.StatusInternalServerError, err
			}

			settingsRepository := repositories.NewGroupReceiptSettingsRepository(nil)
			updated, err := settingsRepository.SetReportTemplate(groupId, fileHeader.Filename, ext)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			responseBytes, err := utils.MarshalResponseData(updated)
			if err != nil {
				return http.StatusInternalServerError, err
			}
			w.WriteHeader(http.StatusOK)
			w.Write(responseBytes)
			return 0, nil
		},
	}
	HandleRequest(handler)
}

// DeleteReportTemplate removes a group's custom report template (file + metadata).
func DeleteReportTemplate(w http.ResponseWriter, r *http.Request) {
	groupId := chi.URLParam(r, "groupId")

	handler := structs.Handler{
		ErrorMessage: "Error deleting report template",
		Writer:       w,
		Request:      r,
		GroupId:      groupId,
		GroupRole:    models.OWNER,
		ResponseType: constants.ApplicationJson,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			group, err := getGroupForTemplate(groupId)
			if err != nil {
				return http.StatusInternalServerError, err
			}
			for _, ext := range []string{"xlsx", "csv"} {
				if path, pErr := utils.BuildReportTemplatePath(groupId, group.Name, ext); pErr == nil {
					_ = os.Remove(path)
				}
			}
			settingsRepository := repositories.NewGroupReceiptSettingsRepository(nil)
			updated, err := settingsRepository.SetReportTemplate(groupId, "", "")
			if err != nil {
				return http.StatusInternalServerError, err
			}
			responseBytes, err := utils.MarshalResponseData(updated)
			if err != nil {
				return http.StatusInternalServerError, err
			}
			w.WriteHeader(http.StatusOK)
			w.Write(responseBytes)
			return 0, nil
		},
	}
	HandleRequest(handler)
}

// ExportReportCustom renders a report into the group's uploaded custom template.
func ExportReportCustom(w http.ResponseWriter, r *http.Request) {
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
			settingsRepository := repositories.NewGroupReceiptSettingsRepository(nil)
			settings, err := settingsRepository.GetGroupReceiptSettings(utils.UintToString(groupId))
			if err != nil {
				return http.StatusInternalServerError, err
			}
			if settings.ReportTemplateType == "" {
				utils.WriteCustomErrorResponse(w, "No custom template configured for this group", http.StatusBadRequest)
				return 0, errors.New("no custom template configured")
			}

			group, err := getGroupForTemplate(utils.UintToString(groupId))
			if err != nil {
				return http.StatusInternalServerError, err
			}
			templatePath, err := utils.BuildReportTemplatePath(utils.UintToString(groupId), group.Name, settings.ReportTemplateType)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			reportRepository := repositories.NewReportRepository(nil)
			report, err := reportRepository.GetReportById(reportId, true)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			templateService := services.NewReportTemplateService(nil)
			fileBytes, kind, err := templateService.BuildFromTemplate(report, settings, templatePath)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			filename := "expense-report." + kind
			contentType := constants.ApplicationXlsx
			if kind == "csv" {
				contentType = constants.TextCsv
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Disposition", "attachment; filename="+filename)
			w.WriteHeader(http.StatusOK)
			w.Write(fileBytes)
			return 0, nil
		},
	}
	HandleRequest(handler)
}

func getGroupForTemplate(groupId string) (models.Group, error) {
	groupRepository := repositories.NewGroupRepository(nil)
	return groupRepository.GetGroupById(groupId, false, false, false)
}

func removeOtherTemplate(groupId string, groupName string, keepExt string) {
	other := "csv"
	if keepExt == "csv" {
		other = "xlsx"
	}
	if path, err := utils.BuildReportTemplatePath(groupId, groupName, other); err == nil {
		_ = os.Remove(path)
	}
}
