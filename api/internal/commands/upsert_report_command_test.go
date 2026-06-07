package commands

import (
	"receipt-wrangler/api/internal/models"
	"testing"
)

func TestUpsertReportCommandRequiresNameAndGroup(t *testing.T) {
	command := UpsertReportCommand{}
	vErr := command.Validate()

	if vErr.Errors["name"] == "" {
		t.Error("expected a name validation error when name is empty")
	}
	if vErr.Errors["groupId"] == "" {
		t.Error("expected a groupId validation error when groupId is zero")
	}
}

func TestUpsertReportCommandRejectsInvalidStatus(t *testing.T) {
	command := UpsertReportCommand{
		Name:    "March",
		GroupId: 1,
		Status:  models.ReportStatus("BOGUS"),
	}
	vErr := command.Validate()

	if vErr.Errors["status"] == "" {
		t.Error("expected a status validation error for an invalid status")
	}
}

func TestUpsertReportCommandAcceptsValidInput(t *testing.T) {
	command := UpsertReportCommand{
		Name:    "March",
		GroupId: 1,
		Status:  models.REPORT_SUBMITTED,
	}
	vErr := command.Validate()

	if len(vErr.Errors) != 0 {
		t.Errorf("expected no validation errors, got %v", vErr.Errors)
	}
}

func TestUpsertReportCommandAllowsEmptyStatus(t *testing.T) {
	command := UpsertReportCommand{
		Name:    "March",
		GroupId: 1,
	}
	vErr := command.Validate()

	if len(vErr.Errors) != 0 {
		t.Errorf("expected no validation errors with empty status, got %v", vErr.Errors)
	}
}
