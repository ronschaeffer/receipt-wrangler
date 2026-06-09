package repositories

import (
	"fmt"
	"strings"
	"gorm.io/gorm"
	"receipt-wrangler/api/internal/commands"
	"receipt-wrangler/api/internal/models"
)

type CategoryRepository struct {
	BaseRepository
}

func NewCategoryRepository(tx *gorm.DB) CategoryRepository {
	repository := CategoryRepository{BaseRepository: BaseRepository{
		DB: GetDB(),
		TX: tx,
	}}
	return repository
}

func (repository CategoryRepository) GetAllCategories(querySelect string) ([]models.Category, error) {
	db := repository.GetDB()
	var categories []models.Category

	err := db.Table("categories").Select(querySelect).Find(&categories).Error
	if err != nil {
		return nil, err
	}

	return categories, nil
}

func (repository CategoryRepository) CreateCategory(category models.Category) (models.Category, error) {
	db := repository.GetDB()

	err := db.Model(&category).Create(&category).Error
	if err != nil {
		return models.Category{}, err
	}

	return category, nil
}

func (repository CategoryRepository) GetAllPagedCategories(pagedRequestCommand commands.PagedRequestCommand) ([]models.CategoryView, error) {
	db := repository.GetDB()
	var categories []models.CategoryView
	quotedAlias := "\"NumberOfReceipts\""

	if pagedRequestCommand.OrderBy == "numberOfReceipts" {
		pagedRequestCommand.OrderBy = quotedAlias
	}

	query := repository.Sort(db, pagedRequestCommand.OrderBy, pagedRequestCommand.SortDirection)
	query = query.Scopes(repository.Paginate(pagedRequestCommand.Page, pagedRequestCommand.PageSize))
	selectString := fmt.Sprintf("categories.id, categories.name, categories.description,  COUNT(DISTINCT receipt_categories.receipt_id) as %s", quotedAlias)
	query = query.Table("categories").
		Select(selectString).
		Joins("LEFT JOIN receipt_categories ON categories.id = receipt_categories.category_id").
		Group("categories.id, categories.name")

	err := query.Scan(&categories).Error
	if err != nil {
		return nil, err
	}

	return categories, nil
}

func (repository CategoryRepository) UpdateCategory(categoryToUpdate models.Category, querySelect string) (models.Category, error) {
	db := repository.GetDB()

	err := db.Model(models.Category{}).Where("id = ?", categoryToUpdate.ID).Updates(map[string]interface{}{"name": categoryToUpdate.Name, "description": categoryToUpdate.Description}).Error
	if err != nil {
		return models.Category{}, err
	}

	return categoryToUpdate, nil
}

func (repository CategoryRepository) DeleteCategory(categoryId uint) error {
	db := repository.GetDB()

	err := db.Transaction(func(tx *gorm.DB) error {
		err := tx.Delete(&models.ReceiptCategory{}, "category_id = ?", categoryId).Error
		if err != nil {
			return err
		}

		err = tx.Where("id = ?", categoryId).Delete(&models.Category{}).Error
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// GetCategoriesForGroup returns the categories a group has enabled (via the
// group_categories join). If the group has enabled none, it falls back to all
// categories — so groups that have not configured a subset keep the prior
// behaviour of seeing every category.
func (repository CategoryRepository) GetCategoriesForGroup(groupId string, querySelect string) ([]models.Category, error) {
	db := repository.GetDB()

	var count int64
	if err := db.Table("group_categories").Where("group_id = ?", groupId).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return repository.GetAllCategories(querySelect)
	}

	// Qualify the requested columns with the categories table so the JOIN does
	// not produce an ambiguous-column error (e.g. "id" exists on both tables).
	qualifiedSelect := qualifyColumns(querySelect, "categories")

	var categories []models.Category
	err := db.Table("categories").
		Select(qualifiedSelect).
		Joins("JOIN group_categories ON group_categories.category_id = categories.id").
		Where("group_categories.group_id = ?", groupId).
		Find(&categories).Error
	if err != nil {
		return nil, err
	}

	return categories, nil
}

// qualifyColumns prefixes each comma-separated column in select with table.,
// unless it already contains a dot or is a wildcard. Used to disambiguate
// columns in joined queries.
func qualifyColumns(querySelect string, table string) string {
	parts := strings.Split(querySelect, ",")
	for i, p := range parts {
		col := strings.TrimSpace(p)
		if col == "" || col == "*" || strings.Contains(col, ".") {
			parts[i] = col
			continue
		}
		parts[i] = table + "." + col
	}
	return strings.Join(parts, ", ")
}

// GetEnabledCategoryIdSet returns the set of category ids enabled for a group.
// An empty result means "no explicit subset configured" (caller should treat as
// all-enabled), which is distinct from a configured-but-empty set; callers that
// need that distinction should check the join count separately.
func (repository CategoryRepository) GetEnabledCategoryIdSet(groupId string) (map[uint]bool, error) {
	db := repository.GetDB()
	var ids []uint
	err := db.Table("group_categories").
		Where("group_id = ?", groupId).
		Pluck("category_id", &ids).Error
	if err != nil {
		return nil, err
	}
	set := make(map[uint]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

// SetGroupCategories replaces a group's enabled-category set with the given ids
// (delete-by-group then re-create), mirroring the child-collection replace
// convention used elsewhere (e.g. group tax rules).
func (repository CategoryRepository) SetGroupCategories(groupId uint, categoryIds []uint) error {
	db := repository.GetDB()

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupId).Delete(&models.GroupCategory{}).Error; err != nil {
			return err
		}
		if len(categoryIds) == 0 {
			return nil
		}
		rows := make([]models.GroupCategory, 0, len(categoryIds))
		for _, cid := range categoryIds {
			rows = append(rows, models.GroupCategory{GroupId: groupId, CategoryId: cid})
		}
		return tx.Create(&rows).Error
	})
}
