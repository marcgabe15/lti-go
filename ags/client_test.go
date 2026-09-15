package ags_test

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
	"github.com/marcgabe15/lti-go/ags"
)

type fakeTokenSource struct {
	token string
	err   error
}

func (f *fakeTokenSource) Token(_ context.Context, _ *lti.Platform, _ []string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

func TestNewClientForLaunch_NotAvailable(t *testing.T) {
	claims := &lti.Claims{} // no AGSEndpoint set
	_, err := ags.NewClientForLaunch(claims, &fakeTokenSource{token: "tok"})
	assert.ErrorIs(t, err, lti.ErrAGSNotAvailable)
}

func newTestClient(t *testing.T, lineItemsURL string) *ags.Client {
	t.Helper()
	claims := &lti.Claims{AGSEndpoint: &lti.AGSEndpointClaim{LineItems: lineItemsURL}}
	client, err := ags.NewClientForLaunch(claims, &fakeTokenSource{token: "test-token"})
	require.NoError(t, err)
	return client
}

func TestClient_LineItemCRUD(t *testing.T) {
	var created ags.LineItem
	mux := http.NewServeMux()
	mux.HandleFunc("/lineitems", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/vnd.ims.lis.v2.lineitemcontainer+json")
			_ = json.NewEncoder(w).Encode([]*ags.LineItem{{ID: "http://example.com/lineitems/1", Label: "Quiz 1", ScoreMaximum: 100}})
		case http.MethodPost:
			require.NoError(t, json.NewDecoder(r.Body).Decode(&created))
			_ = json.NewEncoder(w).Encode(created)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	createdURL := srv.URL + "/lineitems/2"
	mux.HandleFunc("/lineitems/2", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			var li ags.LineItem
			require.NoError(t, json.NewDecoder(r.Body).Decode(&li))
			_ = json.NewEncoder(w).Encode(li)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	})

	client := newTestClient(t, srv.URL+"/lineitems")

	items, err := client.GetLineItems(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Quiz 1", items[0].Label)

	li, err := client.CreateLineItem(context.Background(), &ags.LineItem{Label: "Quiz 2", ScoreMaximum: 50})
	require.NoError(t, err)
	assert.Equal(t, "Quiz 2", li.Label)

	li.ID = createdURL
	li.Label = "Quiz 2 (renamed)"
	updated, err := client.UpdateLineItem(context.Background(), li)
	require.NoError(t, err)
	assert.Equal(t, "Quiz 2 (renamed)", updated.Label)

	require.NoError(t, client.DeleteLineItem(context.Background(), li.ID))
}

func TestClient_LineItems_Pagination(t *testing.T) {
	pages := [][]*ags.LineItem{
		{{ID: "1", Label: "a"}},
		{{ID: "2", Label: "b"}},
	}
	requestCount := 0
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/lineitems", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", fmt.Sprintf(`<%s/lineitems/page2>; rel="next"`, srv.URL))
		_ = json.NewEncoder(w).Encode(pages[0])
		requestCount++
	})
	mux.HandleFunc("/lineitems/page2", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(pages[1])
		requestCount++
	})

	client := newTestClient(t, srv.URL+"/lineitems")
	items, err := client.GetLineItems(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 2, requestCount)
	require.Len(t, items, 2)
	assert.Equal(t, "a", items[0].Label)
	assert.Equal(t, "b", items[1].Label)
}

func TestClient_SubmitScoreAndGetResults(t *testing.T) {
	var gotScore ags.Score
	mux := http.NewServeMux()
	mux.HandleFunc("/lineitems/1/scores", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotScore))
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/lineitems/1/results", func(w http.ResponseWriter, r *http.Request) {
		scoreGiven := 85.0
		_ = json.NewEncoder(w).Encode([]*ags.Result{{ID: "r1", UserID: "user-1", ResultScore: &scoreGiven}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(t, srv.URL+"/lineitems")

	scoreGiven := 85.0
	err := client.SubmitScore(context.Background(), srv.URL+"/lineitems/1", &ags.Score{
		UserID:           "user-1",
		ScoreGiven:       &scoreGiven,
		ActivityProgress: ags.ActivityProgressCompleted,
		GradingProgress:  ags.GradingProgressFullyGraded,
	})
	require.NoError(t, err)
	assert.Equal(t, "user-1", gotScore.UserID)

	results, err := client.GetResults(context.Background(), srv.URL+"/lineitems/1", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "user-1", results[0].UserID)
}
