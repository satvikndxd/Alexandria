package httpapi

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// handlePushConfig publishes the VAPID public key so the browser can create
// a subscription. The public key is not a secret; the private half never
// leaves server configuration.
func (s *Server) handlePushConfig(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"enabled":    s.pushCfg.Enabled(),
		"public_key": s.pushCfg.PublicKey,
	})
}

type pushSubscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req pushSubscribeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !s.pushCfg.Enabled() {
		respondError(w, http.StatusServiceUnavailable, "push_disabled",
			"This instance has no push keys configured.")
		return
	}
	p256dh, err := decodeKey(req.Keys.P256DH)
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_keys", "p256dh is not base64url.")
		return
	}
	auth, err := decodeKey(req.Keys.Auth)
	if err != nil || len(auth) != 16 {
		respondError(w, http.StatusBadRequest, "bad_keys", "auth must be 16 base64url bytes.")
		return
	}
	sub, err := s.store.AddPushSubscription(r.Context(), sess.UserID, req.Endpoint, p256dh, auth, r.UserAgent())
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"subscription": sub.ID})
}

func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.DeletePushSubscription(r.Context(), sess.UserID, req.Endpoint); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "unsubscribed"})
}

// decodeKey accepts padded or unpadded base64url, as browsers emit both.
func decodeKey(s string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "=")); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}
