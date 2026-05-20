package prometheus

import "net/http"

// auth is the strategy applied to outbound requests.
type auth interface {
	apply(*http.Request)
}

type noAuth struct{}

func (noAuth) apply(*http.Request) {}

type bearerAuth struct{ token string }

func (b bearerAuth) apply(r *http.Request) {
	r.Header.Set("Authorization", "Bearer "+b.token)
}

type basicAuth struct{ user, pass string }

func (b basicAuth) apply(r *http.Request) {
	r.SetBasicAuth(b.user, b.pass)
}
