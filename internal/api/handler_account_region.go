package api

import (
	"net/http"
	"strings"

	"github.com/Resinat/Resin/internal/service"
)

// HandleListAccountRegions returns a handler for GET /api/v1/platforms/{id}/account-regions.
func HandleListAccountRegions(cp *service.ControlPlaneService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		platformID := PathParam(r, "id")
		if !ValidateUUID(platformID) {
			WriteError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid platform id")
			return
		}

		regions, err := cp.ListAccountRegions(platformID)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		account := strings.TrimSpace(r.URL.Query().Get("account"))
		if account != "" {
			filtered := make([]service.AccountRegionResponse, 0, len(regions))
			for _, ar := range regions {
				if ar.Account == account {
					filtered = append(filtered, ar)
				}
			}
			regions = filtered
		}

		pg, err := ParsePagination(r)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
			return
		}
		WritePage(w, http.StatusOK, regions, pg)
	}
}

// HandleDeleteAccountRegion returns a handler for DELETE /api/v1/platforms/{id}/account-regions/{account}.
func HandleDeleteAccountRegion(cp *service.ControlPlaneService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		platformID := PathParam(r, "id")
		if !ValidateUUID(platformID) {
			WriteError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid platform id")
			return
		}
		account, err := validateAccountPath(r)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		if err := cp.DeleteAccountRegion(platformID, account); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleDeleteAllAccountRegions returns a handler for DELETE /api/v1/platforms/{id}/account-regions.
func HandleDeleteAllAccountRegions(cp *service.ControlPlaneService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		platformID := PathParam(r, "id")
		if !ValidateUUID(platformID) {
			WriteError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid platform id")
			return
		}

		count, err := cp.DeleteAllAccountRegions(platformID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]int{"deleted_count": count})
	}
}
