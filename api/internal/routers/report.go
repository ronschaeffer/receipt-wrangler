package routers

import (
	"receipt-wrangler/api/internal/handlers"
	"receipt-wrangler/api/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func BuildReportRouter() *chi.Mux {
	reportRouter := chi.NewRouter()

	reportRouter.Use(middleware.UnifiedAuthMiddleware)

	reportRouter.Post("/", handlers.CreateReport)
	reportRouter.Get("/group/{groupId}", handlers.GetReportsForGroup)
	reportRouter.Get("/{reportId}", handlers.GetReport)
	reportRouter.Put("/{reportId}", handlers.UpdateReport)
	reportRouter.Delete("/{reportId}", handlers.DeleteReport)

	reportRouter.Post("/{reportId}/receipts", handlers.AddReceiptsToReport)
	reportRouter.Delete("/{reportId}/receipts", handlers.RemoveReceiptsFromReport)

	reportRouter.Post("/{reportId}/export/csv", handlers.ExportReportCsv)
	reportRouter.Post("/{reportId}/export/pack", handlers.ExportReportReceiptPack)
	reportRouter.Post("/{reportId}/export/xlsx", handlers.ExportReportXlsx)
	reportRouter.Post("/{reportId}/export/custom", handlers.ExportReportCustom)

	return reportRouter
}
