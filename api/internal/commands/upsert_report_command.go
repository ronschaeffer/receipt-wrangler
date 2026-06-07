package commands

import (
	"encoding/json"
	"net/http"
	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/structs"
	"receipt-wrangler/api/internal/utils"
)

type UpsertReportCommand struct {
	Name    string              `json:"name"`
	GroupId uint                `json:"groupId"`
	Status  models.ReportStatus `json:"status"`
}

func (command *UpsertReportCommand) LoadDataFromRequest(w http.ResponseWriter, r *http.Request) error {
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

func (command *UpsertReportCommand) Validate() structs.ValidatorError {
	errorMap := make(map[string]string)
	vErr := structs.ValidatorError{}

	if len(command.Name) == 0 {
		errorMap["name"] = "Name is required"
	}

	if command.GroupId == 0 {
		errorMap["groupId"] = "Group is required"
	}

	if len(command.Status) > 0 && !utils.Contains(models.ReportStatuses(), command.Status) {
		errorMap["status"] = "Invalid status"
	}

	vErr.Errors = errorMap
	return vErr
}
