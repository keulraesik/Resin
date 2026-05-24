package service

import (
	"errors"
	"time"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/state"
)

// AccountRegionResponse is the API response for account primary-region affinity.
type AccountRegionResponse struct {
	PlatformID    string `json:"platform_id"`
	Account       string `json:"account"`
	PrimaryRegion string `json:"primary_region"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func accountRegionToResponse(ar model.AccountRegion) AccountRegionResponse {
	return AccountRegionResponse{
		PlatformID:    ar.PlatformID,
		Account:       ar.Account,
		PrimaryRegion: ar.PrimaryRegion,
		CreatedAt:     time.Unix(0, ar.CreatedAtNs).UTC().Format(time.RFC3339Nano),
		UpdatedAt:     time.Unix(0, ar.UpdatedAtNs).UTC().Format(time.RFC3339Nano),
	}
}

// ListAccountRegions returns all account primary regions for a platform.
func (s *ControlPlaneService) ListAccountRegions(platformID string) ([]AccountRegionResponse, error) {
	if _, err := s.getPlatformModel(platformID); err != nil {
		return nil, err
	}
	regions, err := s.Engine.ListAccountRegions(platformID)
	if err != nil {
		return nil, internal("list account regions", err)
	}
	resp := make([]AccountRegionResponse, len(regions))
	for i, ar := range regions {
		resp[i] = accountRegionToResponse(ar)
	}
	return resp, nil
}

// DeleteAccountRegion clears one account primary-region affinity row.
func (s *ControlPlaneService) DeleteAccountRegion(platformID, account string) error {
	if _, err := s.getPlatformModel(platformID); err != nil {
		return err
	}
	if err := s.Engine.DeleteAccountRegion(platformID, account); err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return notFound("account region not found")
		}
		return internal("delete account region", err)
	}
	return nil
}

// DeleteAllAccountRegions clears all account primary-region affinity rows for a platform.
func (s *ControlPlaneService) DeleteAllAccountRegions(platformID string) (int, error) {
	if _, err := s.getPlatformModel(platformID); err != nil {
		return 0, err
	}
	count, err := s.Engine.DeleteAccountRegionsByPlatform(platformID)
	if err != nil {
		return 0, internal("delete account regions", err)
	}
	return count, nil
}
