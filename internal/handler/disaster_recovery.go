package handler

import "net/http"

func (h *Handler) GetDisasterRecoveryScore(w http.ResponseWriter, r *http.Request) {
	if h.db == nil || h.disasterRecovery == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "disaster recovery service unavailable")
		return
	}

	instanceID, err := instanceIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid instance id")
		return
	}

	if _, err := h.db.GetInstanceByID(instanceID); err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	score, err := h.disasterRecovery.GetScore(r.Context(), instanceID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to calculate disaster recovery score")
		return
	}

	JSON(w, http.StatusOK, score)
}
