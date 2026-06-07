package models

import (
	"database/sql/driver"
	"errors"
)

type ReportStatus string

const (
	REPORT_DRAFT     ReportStatus = "DRAFT"
	REPORT_SUBMITTED ReportStatus = "SUBMITTED"
	REPORT_PAID      ReportStatus = "PAID"
)

func (self *ReportStatus) Scan(value string) error {
	*self = ReportStatus(value)
	return nil
}

func (self ReportStatus) Value() (driver.Value, error) {
	if self != REPORT_DRAFT && self != REPORT_SUBMITTED && self != REPORT_PAID && self != "" {
		return nil, errors.New("invalid reportStatus")
	}
	return string(self), nil
}

func ReportStatuses() []interface{} {
	return []interface{}{REPORT_DRAFT, REPORT_SUBMITTED, REPORT_PAID}
}
