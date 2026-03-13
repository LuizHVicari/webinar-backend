package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LuizHVicari/webinar-backend/pkg/middleware"
	"github.com/LuizHVicari/webinar-backend/pkg/testhelper"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeUserResolver returns a fixed user ID and no org for any identity.
type fakeUserResolver struct {
	userID uuid.UUID
}

func (f *fakeUserResolver) GetOrCreate(_ context.Context, _ uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
	return f.userID, nil, nil
}

// createIdentity calls the Kratos admin API to create a new identity with the given email and password.
// Returns the identity ID.
func createIdentity(t *testing.T, adminURL, email, password string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]any{
		"schema_id": "default",
		"traits":    map[string]any{"email": email},
		"credentials": map[string]any{
			"password": map[string]any{
				"config": map[string]any{
					"password": password,
				},
			},
		},
	})
	resp, err := http.Post(adminURL+"/identities", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result["id"].(string)
}

// loginAndGetSessionCookie performs the Kratos browser login self-service flow and returns
// the value of the ory_kratos_session cookie set by Kratos on successful authentication.
func loginAndGetSessionCookie(t *testing.T, publicURL, email, password string) string {
	t.Helper()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	// Do not follow redirects automatically so we can control the flow.
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Step 1: initiate browser login flow. Kratos sets a CSRF cookie and redirects to the UI URL.
	initResp, err := client.Get(publicURL + "/self-service/login/browser")
	require.NoError(t, err)
	_ = initResp.Body.Close()

	location := initResp.Header.Get("Location")
	require.NotEmpty(t, location, "expected redirect from /self-service/login/browser")

	parsedLoc, err := url.Parse(location)
	require.NoError(t, err)
	flowID := parsedLoc.Query().Get("flow")
	require.NotEmpty(t, flowID, "flow ID not in Location header: %s", location)

	// Step 2: fetch the flow to get the CSRF token.
	flowResp, err := client.Get(publicURL + "/self-service/login/flows?id=" + flowID)
	require.NoError(t, err)
	defer func() { _ = flowResp.Body.Close() }()

	var flowData map[string]any
	require.NoError(t, json.NewDecoder(flowResp.Body).Decode(&flowData))

	csrfToken := extractCSRFToken(flowData)

	// Step 3: submit credentials. Kratos sets ory_kratos_session in the response cookies.
	formData := url.Values{
		"method":     {"password"},
		"identifier": {email},
		"password":   {password},
		"csrf_token": {csrfToken},
	}
	submitResp, err := client.PostForm(publicURL+"/self-service/login?flow="+flowID, formData)
	require.NoError(t, err)
	_ = submitResp.Body.Close()

	// Step 4: retrieve the session cookie from the jar.
	parsedURL, _ := url.Parse(publicURL)
	for _, cookie := range jar.Cookies(parsedURL) {
		if cookie.Name == "ory_kratos_session" {
			return cookie.Value
		}
	}
	t.Fatal("ory_kratos_session cookie not found after login")
	return ""
}

func extractCSRFToken(flowData map[string]any) string {
	ui, _ := flowData["ui"].(map[string]any)
	nodes, _ := ui["nodes"].([]any)
	for _, node := range nodes {
		n, _ := node.(map[string]any)
		attrs, _ := n["attributes"].(map[string]any)
		if attrs["name"] == "csrf_token" {
			v, _ := attrs["value"].(string)
			return v
		}
	}
	return ""
}

func TestAuth_ValidSession_PopulatesContext(t *testing.T) {
	testhelper.DeleteAllIdentities(t, sharedKratos.AdminURL)

	const (
		email    = "test@example.com"
		password = "valid-test-password-123!"
	)

	userID, err := uuid.NewV7()
	require.NoError(t, err)
	resolver := &fakeUserResolver{userID: userID}

	identityID := createIdentity(t, sharedKratos.AdminURL, email, password)
	sessionCookie := loginAndGetSessionCookie(t, sharedKratos.PublicURL, email, password)

	r := gin.New()
	r.Use(middleware.Auth(sharedKratos.PublicURL, resolver))
	r.GET("/me", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":        c.GetString(middleware.ContextKeyUserID),
			"identity_email": c.GetString(middleware.ContextKeyIdentityEmail),
			"identity_id":    c.GetString(middleware.ContextKeyIdentityID),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: "ory_kratos_session", Value: sessionCookie})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, userID.String(), result["user_id"])
	assert.Equal(t, email, result["identity_email"])
	assert.Equal(t, identityID, result["identity_id"])
}

func TestAuth_MissingCookie_Returns401(t *testing.T) {
	resolver := &fakeUserResolver{}

	r := gin.New()
	r.Use(middleware.Auth(sharedKratos.PublicURL, resolver))
	r.GET("/me", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_InvalidSessionToken_Returns401(t *testing.T) {
	resolver := &fakeUserResolver{}

	r := gin.New()
	r.Use(middleware.Auth(sharedKratos.PublicURL, resolver))
	r.GET("/me", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: "ory_kratos_session", Value: "not-a-valid-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
