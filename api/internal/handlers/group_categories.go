package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"receipt-wrangler/api/internal/constants"
	"receipt-wrangler/api/internal/models"
	"receipt-wrangler/api/internal/repositories"
	"receipt-wrangler/api/internal/structs"
	"receipt-wrangler/api/internal/utils"
)

// GetGroupCategories returns the categories enabled for a group. If the group
// has no configured subset, this returns all categories (the fallback).
func GetGroupCategories(w http.ResponseWriter, r *http.Request) {
	groupId := chi.URLParam(r, "groupId")

	handler := structs.Handler{
		ErrorMessage: "Error retrieving group categories",
		Writer:       w,
		Request:      r,
		ResponseType: constants.ApplicationJson,
		GroupId:      groupId,
		GroupRole:    models.VIEWER,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			categoryRepository := repositories.NewCategoryRepository(nil)
			categories, err := categoryRepository.GetCategoriesForGroup(groupId, "id, name, description")
			if err != nil {
				return http.StatusInternalServerError, err
			}

			bytes, err := utils.MarshalResponseData(categories)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			w.WriteHeader(http.StatusOK)
			w.Write(bytes)
			return 0, nil
		},
	}

	HandleRequest(handler)
}

// SetGroupCategoriesCommand is the request body for SetGroupCategories.
type SetGroupCategoriesCommand struct {
	CategoryIds []uint `json:"categoryIds"`
}

// SetGroupCategories replaces a group's enabled-category set. An empty list
// clears the subset, which restores the "all categories" fallback for the group.
func SetGroupCategories(w http.ResponseWriter, r *http.Request) {
	groupId := chi.URLParam(r, "groupId")

	handler := structs.Handler{
		ErrorMessage: "Error updating group categories",
		Writer:       w,
		Request:      r,
		ResponseType: constants.ApplicationJson,
		GroupId:      groupId,
		GroupRole:    models.OWNER,
		HandlerFunction: func(w http.ResponseWriter, r *http.Request) (int, error) {
			bodyBytes, err := utils.GetBodyData(w, r)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			command := SetGroupCategoriesCommand{}
			if err := json.Unmarshal(bodyBytes, &command); err != nil {
				return http.StatusBadRequest, err
			}

			uintGroupId, err := utils.StringToUint(groupId)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			categoryRepository := repositories.NewCategoryRepository(nil)
			if err := categoryRepository.SetGroupCategories(uintGroupId, command.CategoryIds); err != nil {
				return http.StatusInternalServerError, err
			}

			categories, err := categoryRepository.GetCategoriesForGroup(groupId, "id, name, description")
			if err != nil {
				return http.StatusInternalServerError, err
			}

			bytes, err := utils.MarshalResponseData(categories)
			if err != nil {
				return http.StatusInternalServerError, err
			}

			w.WriteHeader(http.StatusOK)
			w.Write(bytes)
			return 0, nil
		},
	}

	HandleRequest(handler)
}
