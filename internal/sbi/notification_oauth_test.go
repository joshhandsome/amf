package sbi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/pkg/factory"
)

func TestCallbackRoutesWithoutOAuthToken(t *testing.T) {
	self := amf_context.GetSelf()
	oldOAuth := self.OAuth2Required.Load()
	oldConfig := factory.AmfConfig
	self.OAuth2Required.Store(true)
	factory.AmfConfig = &factory.Config{Configuration: &factory.Configuration{ServiceNameList: []string{"namf-comm"}}}
	t.Cleanup(func() { self.OAuth2Required.Store(oldOAuth); factory.AmfConfig = oldConfig })
	router := newRouter(&Server{})
	for _, path := range []string{
		"/am-policy/test/update", "/am-policy/test/terminate", "/smContextStatus/test/1",
		"/n1-message-notify", "/deregistration/test",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodPost, factory.AmfCallbackResUriPrefix+path, strings.NewReader("{"),
			)
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			// The real callback handler must process/reject the body, rather than an OAuth middleware rejecting it.
			require.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
	t.Run("normal service still requires OAuth", func(t *testing.T) {
		response := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(), http.MethodPost, factory.AmfCommResUriPrefix+"/ue-contexts/test/n1-n2-messages", nil,
		)
		router.ServeHTTP(response, req)
		require.Equal(t, http.StatusUnauthorized, response.Code)
	})
}
