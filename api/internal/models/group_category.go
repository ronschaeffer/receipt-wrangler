package models

// GroupCategory is a join between a group and a (globally-defined) category,
// recording which categories a group has "enabled". Categories remain global;
// this table lets each group expose only a chosen subset for receipt
// classification and AI suggestion. A group with no rows here is treated as
// "all categories enabled" (preserving pre-feature behaviour).
type GroupCategory struct {
	BaseModel
	GroupId    uint `gorm:"not null;index;uniqueIndex:idx_group_category" json:"groupId"`
	CategoryId uint `gorm:"not null;index;uniqueIndex:idx_group_category" json:"categoryId"`
}
