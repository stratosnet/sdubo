package sds

import "net/http"

const (
	SpfsPrivateKeyHeader = "X-Auth-Privatekey"
	SpfsUserIdHeader     = "X-Auth-UserId"
	OptionSpfsPrivKey    = "spfs-privKey"
	OptionSpfsUserId     = "spfs-userId"
)

// sgMiddleware is an http middleware which wraps the next handler for sg headers recovery
type sgMiddleware struct {
}

// the internal service gateway handler for the API
type sgHandler struct {
}

// NewHandler creates the http.Handler to be able to work with service gateway
func NewSGHandler(handler http.Handler) http.Handler {
	return NewSGMiddleware()(handler)
}

// NewSGMiddleware returns a service gateway header capture instrumentation middleware.
func NewSGMiddleware() func(http.Handler) http.Handler {
	h := &sgMiddleware{}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.serveHTTP(w, r, next)
		})
	}
}

func (h *sgMiddleware) serveHTTP(w http.ResponseWriter, r *http.Request, next http.Handler) {
	privKey := r.Header.Get(SpfsPrivateKeyHeader)
	userId := r.Header.Get(SpfsUserIdHeader)

	r = r.Clone(r.Context())
	query := r.URL.Query()
	{
		if privKey != "" {
			query.Set(OptionSpfsPrivKey, privKey)
		}
		if userId != "" {
			query.Set(OptionSpfsUserId, userId)
		}
	}
	r.URL.RawQuery = query.Encode()

	next.ServeHTTP(w, r)
}
