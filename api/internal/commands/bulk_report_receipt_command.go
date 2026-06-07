package commands

import (
	"encoding/json"
	"net/http"
	"receipt-wrangler/api/internal/utils"
)

// BulkReportReceiptCommand assigns or removes a set of receipts to/from a
// report in one operation, mirroring the select-many gesture already used by
// BulkStatusUpdateCommand. The receipt IDs come from the same multi-select the
// receipt list uses for bulk status updates.
type BulkReportReceiptCommand struct {
	ReceiptIds []uint `json:"receiptIds"`
}

func (command *BulkReportReceiptCommand) LoadDataFromRequest(w http.ResponseWriter, r *http.Request) error {
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
