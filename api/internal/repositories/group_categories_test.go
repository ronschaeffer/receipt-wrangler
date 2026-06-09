package repositories

import (
	"testing"

	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/utils"
)

func setupGroupCategoryTest(t *testing.T) (uint, []models.Category) {
	t.Helper()
	db := GetDB()

	group := models.Group{Name: "GC Test Group"}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}

	cats := []models.Category{
		{Name: "Travel"},
		{Name: "Subsistence"},
		{Name: "Components and cables"},
	}
	for i := range cats {
		if err := db.Create(&cats[i]).Error; err != nil {
			t.Fatalf("create category: %v", err)
		}
	}
	return group.ID, cats
}

func TestGroupCategories_FallbackToAllWhenNoneConfigured(t *testing.T) {
	defer TruncateTestDb()
	groupId, cats := setupGroupCategoryTest(t)
	repo := NewCategoryRepository(nil)

	got, err := repo.GetCategoriesForGroup(utils.UintToString(groupId), "id, name")
	if err != nil {
		t.Fatalf("GetCategoriesForGroup: %v", err)
	}
	if len(got) != len(cats) {
		t.Errorf("expected fallback to all %d categories, got %d", len(cats), len(got))
	}
}

func TestGroupCategories_SubsetWhenConfigured(t *testing.T) {
	defer TruncateTestDb()
	groupId, cats := setupGroupCategoryTest(t)
	repo := NewCategoryRepository(nil)

	// Enable only Travel + Subsistence (exclude "Components and cables").
	if err := repo.SetGroupCategories(groupId, []uint{cats[0].ID, cats[1].ID}); err != nil {
		t.Fatalf("SetGroupCategories: %v", err)
	}

	got, err := repo.GetCategoriesForGroup(utils.UintToString(groupId), "id, name")
	if err != nil {
		t.Fatalf("GetCategoriesForGroup: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 enabled categories, got %d", len(got))
	}
	names := map[string]bool{}
	for _, c := range got {
		names[c.Name] = true
	}
	if names["Components and cables"] {
		t.Errorf("excluded category leaked into group set")
	}
	if !names["Travel"] || !names["Subsistence"] {
		t.Errorf("expected Travel + Subsistence, got %v", names)
	}
}

func TestGroupCategories_EnabledIdSet(t *testing.T) {
	defer TruncateTestDb()
	groupId, cats := setupGroupCategoryTest(t)
	repo := NewCategoryRepository(nil)

	if err := repo.SetGroupCategories(groupId, []uint{cats[0].ID}); err != nil {
		t.Fatalf("SetGroupCategories: %v", err)
	}
	set, err := repo.GetEnabledCategoryIdSet(utils.UintToString(groupId))
	if err != nil {
		t.Fatalf("GetEnabledCategoryIdSet: %v", err)
	}
	if !set[cats[0].ID] || set[cats[2].ID] {
		t.Errorf("enabled-id set wrong: %v", set)
	}
}

func TestGroupCategories_ReplaceSemantics(t *testing.T) {
	defer TruncateTestDb()
	groupId, cats := setupGroupCategoryTest(t)
	repo := NewCategoryRepository(nil)

	if err := repo.SetGroupCategories(groupId, []uint{cats[0].ID, cats[1].ID}); err != nil {
		t.Fatalf("first set: %v", err)
	}
	// Replace with a different single category.
	if err := repo.SetGroupCategories(groupId, []uint{cats[2].ID}); err != nil {
		t.Fatalf("replace set: %v", err)
	}
	got, err := repo.GetCategoriesForGroup(utils.UintToString(groupId), "id, name")
	if err != nil {
		t.Fatalf("GetCategoriesForGroup: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Components and cables" {
		t.Errorf("replace did not take effect, got %v", got)
	}
}
