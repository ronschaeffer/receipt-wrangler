package repositories

import (
	"os"
	"path/filepath"
	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReceiptImageRepository struct {
	BaseRepository
}

func NewReceiptImageRepository(tx *gorm.DB) ReceiptImageRepository {
	repository := ReceiptImageRepository{BaseRepository: BaseRepository{
		DB: GetDB(),
		TX: tx,
	}}
	return repository
}

// TODO: Move to service
func (repository ReceiptImageRepository) CreateReceiptImage(fileData models.FileData, fileBytes []byte) (models.FileData, error) {
	fileRepository := NewFileRepository(repository.TX)
	db := repository.GetDB()

	// TODO: refactor to use command
	validatedFileType, err := fileRepository.ValidateFileType(fileBytes)
	if err != nil {
		return models.FileData{}, err
	}

	fileData.FileType = validatedFileType

	// Compute a content hash of the uploaded bytes for exact-duplicate detection.
	sourceHash := utils.Sha256Hash(fileBytes)
	fileData.SourceHash = &sourceHash

	basePath, err := os.Getwd()
	if err != nil {
		return models.FileData{}, err
	}

	// Check if data path exists
	err = utils.DirectoryExists(basePath+"/data", true)
	if err != nil {
		return models.FileData{}, err
	}

	// Get initial group directory to see if it exists
	filePath, err := fileRepository.BuildFilePath(utils.UintToString(fileData.ReceiptId), "", fileData.Name)
	if err != nil {
		return models.FileData{}, err
	}
	groupDir, _ := filepath.Split(filePath)

	err = db.Model(models.FileData{}).Create(&fileData).Error
	if err != nil {
		os.Remove(filePath)
		return models.FileData{}, err
	}

	// Check if group's path exists
	err = utils.DirectoryExists(groupDir, true)
	if err != nil {
		return models.FileData{}, err
	}

	// Rebuild file path with correct file id
	filePath, err = fileRepository.BuildFilePath(utils.UintToString(fileData.ReceiptId), utils.UintToString(fileData.ID), fileData.Name)
	if err != nil {
		return models.FileData{}, err
	}

	err = utils.WriteFile(filePath, fileBytes)
	if err != nil {
		return models.FileData{}, err
	}

	// Best-effort exact-duplicate detection: if another receipt in the same group
	// already has a file with this hash, flag the new receipt for review. Never
	// blocks the upload.
	repository.flagIfDuplicate(fileData, sourceHash)

	return fileData, nil
}

// flagIfDuplicate looks for an existing receipt (other than this one) in the same
// group whose file shares the given content hash. If found, the new receipt is
// set to NEEDS_ATTENTION and a comment is added linking the original. Errors are
// swallowed so a detection failure never breaks the upload.
func (repository ReceiptImageRepository) flagIfDuplicate(newFile models.FileData, sourceHash string) {
	db := repository.GetDB()

	// Resolve the new file's receipt + group.
	var newReceipt models.Receipt
	if err := db.Model(&models.Receipt{}).Select("id, group_id").Where("id = ?", newFile.ReceiptId).First(&newReceipt).Error; err != nil {
		return
	}

	// Find another receipt in the same group with a file of the same hash.
	var original models.Receipt
	err := db.Model(&models.Receipt{}).
		Joins("JOIN file_data ON file_data.receipt_id = receipts.id").
		Where("file_data.source_hash = ?", sourceHash).
		Where("receipts.group_id = ?", newReceipt.GroupId).
		Where("receipts.id <> ?", newReceipt.ID).
		Select("receipts.id, receipts.group_id, receipts.name").
		Order("receipts.id asc").
		First(&original).Error
	if err != nil {
		return // no duplicate (or lookup failed) — nothing to do
	}

	// Flag the new receipt and leave a breadcrumb to the original.
	db.Model(&models.Receipt{}).Where("id = ?", newReceipt.ID).Update("status", models.NEEDS_ATTENTION)

	comment := models.Comment{
		ReceiptId:      newReceipt.ID,
		Comment:        "Possible duplicate: an existing receipt in this group has an identical file.",
		AdditionalInfo: "duplicate-of-receipt:" + utils.UintToString(original.ID),
	}
	db.Model(&models.Comment{}).Create(&comment)
}

func (repository ReceiptImageRepository) GetReceiptImageById(receiptImageId uint) (models.FileData, error) {
	db := repository.GetDB()
	var result models.FileData

	err := db.Model(models.FileData{}).Where("id = ?", receiptImageId).Preload(clause.Associations).Find(&result).Error
	if err != nil {
		return models.FileData{}, err
	}

	return result, nil
}

func (repository ReceiptImageRepository) GetReceiptImagesByIdArray(receiptImageIds []uint) ([]models.FileData, error) {
	db := repository.GetDB()
	var result = make([]models.FileData, 0)

	err := db.Model(models.FileData{}).Where("id IN ?", receiptImageIds).Find(&result).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}
