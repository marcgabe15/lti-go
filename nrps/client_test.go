package nrps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/nrps"
)

type fakeTokenSource struct{ token string }

func (f *fakeTokenSource) Token(_ context.Context, _ *lti.Platform, scopes []string) (string, error) {
	if len(scopes) != 1 || scopes[0] != nrps.ScopeContextMembershipReadonly {
		return "", fmt.Errorf("unexpected scopes: %v", scopes)
	}
	return f.token, nil
}

func TestNewClientForLaunch_NotAvailable(t *testing.T) {
	claims := &lti.Claims{} // no NRPSEndpoint set
	_, err := nrps.NewClientForLaunch(claims, &fakeTokenSource{token: "tok"})
	assert.ErrorIs(t, err, lti.ErrNRPSNotAvailable)
}

func newTestClient(t *testing.T, membershipsURL string) *nrps.Client {
	t.Helper()
	claims := &lti.Claims{NRPSEndpoint: &lti.NRPSEndpointClaim{ContextMembershipsURL: membershipsURL}}
	client, err := nrps.NewClientForLaunch(claims, &fakeTokenSource{token: "test-token"})
	require.NoError(t, err)
	return client
}

func TestClient_GetMembers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/memberships", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/vnd.ims.lti-nrps.v2.membershipcontainer+json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "https://platform.example.com/memberships",
			"members": []map[string]any{
				{"user_id": "user-1", "roles": []string{"Learner"}, "name": "Ada Lovelace"},
				{"user_id": "user-2", "roles": []string{"Instructor"}, "name": "Alan Turing"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(t, srv.URL+"/memberships")
	members, err := client.GetMembers(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, members, 2)
	assert.Equal(t, "user-1", members[0].UserID)
	assert.Equal(t, []string{"Learner"}, members[0].Roles)
	assert.Equal(t, "Alan Turing", members[1].Name)
}

func TestClient_GetMembers_Pagination(t *testing.T) {
	requestCount := 0
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/memberships", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Link", fmt.Sprintf(`<%s/memberships/page2>; rel="next"`, srv.URL))
		_ = json.NewEncoder(w).Encode(map[string]any{"members": []map[string]any{{"user_id": "user-1", "roles": []string{"Learner"}}}})
	})
	mux.HandleFunc("/memberships/page2", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		_ = json.NewEncoder(w).Encode(map[string]any{"members": []map[string]any{{"user_id": "user-2", "roles": []string{"Learner"}}}})
	})

	client := newTestClient(t, srv.URL+"/memberships")
	members, err := client.GetMembers(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 2, requestCount)
	require.Len(t, members, 2)
	assert.Equal(t, "user-1", members[0].UserID)
	assert.Equal(t, "user-2", members[1].UserID)
}

func TestClient_GetMembers_MaxPages(t *testing.T) {
	requestCount := 0
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/memberships", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Link", fmt.Sprintf(`<%s/memberships>; rel="next"`, srv.URL)) // would loop forever without MaxPages
		_ = json.NewEncoder(w).Encode(map[string]any{"members": []map[string]any{{"user_id": "user-1", "roles": []string{"Learner"}}}})
	})

	client := newTestClient(t, srv.URL+"/memberships")
	members, err := client.GetMembers(context.Background(), &nrps.GetMembersParams{MaxPages: 3})
	require.NoError(t, err)
	assert.Equal(t, 3, requestCount)
	assert.Len(t, members, 3)
}
