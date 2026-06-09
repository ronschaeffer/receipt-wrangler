package models

type FileData struct {
	BaseModel
	Name      string  `json:"name"`
	FileType  string  `json:"fileType"`
	Size      uint    `json:"size"`
	ReceiptId uint    `json:"receiptId"`
	Receipt   Receipt `json:"-"`
	// SourceHash is the SHA-256 hex digest of the uploaded file bytes, used for
	// exact-duplicate detection on ingest. Nullable for legacy rows.
	SourceHash *string `gorm:"type:varchar(64);index" json:"sourceHash,omitempty"`
}
