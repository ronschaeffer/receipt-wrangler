package models

import (
	"encoding/json"
	"net/http"
	"receipt-wrangler/api/internal/utils"
	"time"
)

// Report is a named, group-scoped collection of receipts assembled for
// submission as a company expense claim. Receipts keep their single GroupId
// untouched; membership in a report is a many-to-many association via
// report_receipts, so a receipt may belong to a report (or several) without
// affecting its group placement or its on-disk image path.
type Report struct {
	BaseModel
	Name          string       `gorm:"not null" json:"name"`
	GroupId       uint         `gorm:"not null" json:"groupId"`
	Group         Group        `json:"-"`
	Status        ReportStatus `gorm:"default:'DRAFT';not null" json:"status"`
	SubmittedDate *time.Time   `json:"submittedDate"`
	PaidDate      *time.Time   `json:"paidDate"`
	Receipts      []Receipt    `gorm:"many2many:report_receipts" json:"receipts"`
}

func (report *Report) LoadDataFromRequest(w http.ResponseWriter, r *http.Request) error {
	bytes, err := utils.GetBodyData(w, r)
	if err != nil {
		return err
	}

	err = json.Unmarshal(bytes, &report)
	if err != nil {
		return err
	}

	return nil
}
