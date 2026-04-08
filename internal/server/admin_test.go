package server

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockK8sClient is a mock implementation of the k8s.K8sClient interface for testing.
type MockK8sClient struct {
	isMaintenance    bool
	isMaintError     error
	setMaintError    error
	ensureSvcError   error
}

// EnsureService is a mock implementation of EnsureService.
func (m *MockK8sClient) EnsureService(ctx context.Context, name string, port, targetPort int32, labels map[string]string) error {
	return m.ensureSvcError
}

// IsMaintenanceMode is a mock implementation of IsMaintenanceMode.
func (m *MockK8sClient) IsMaintenanceMode(ctx context.Context, routeName, maintenanceServiceName string) (bool, error) {
	return m.isMaintenance, m.isMaintError
}

// SetMaintenanceMode is a mock implementation of SetMaintenanceMode.
func (m *MockK8sClient) SetMaintenanceMode(ctx context.Context, routeName, targetServiceName string, targetPort int32) error {
	if m.setMaintError != nil {
		return m.setMaintError
	}
	if targetServiceName == "maintenance-page" {
		m.isMaintenance = true
	} else {
		m.isMaintenance = false
	}
	return nil
}

func TestHandleToggle(t *testing.T) {
	e := echo.New()

	t.Run("success", func(t *testing.T) {
		mockK8s := &MockK8sClient{isMaintenance: false}
		app, err := NewAdminApp(mockK8s, "route", "maintenance-page", "de-ui", 80, 80, "../../public/admin.html", logrus.New())
		require.NoError(t, err)

		// First toggle: OFF -> ON
		req := httptest.NewRequest(http.MethodPost, "/toggle", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, app.HandleToggle(c)) {
			assert.Equal(t, http.StatusSeeOther, rec.Code)
			assert.True(t, mockK8s.isMaintenance)
		}

		// Second toggle: ON -> OFF
		req = httptest.NewRequest(http.MethodPost, "/toggle", nil)
		rec = httptest.NewRecorder()
		c = e.NewContext(req, rec)

		if assert.NoError(t, app.HandleToggle(c)) {
			assert.Equal(t, http.StatusSeeOther, rec.Code)
			assert.False(t, mockK8s.isMaintenance)
		}
	})

	t.Run("IsMaintenanceMode error", func(t *testing.T) {
		mockK8s := &MockK8sClient{isMaintError: fmt.Errorf("k8s error")}
		app, err := NewAdminApp(mockK8s, "route", "maintenance-page", "de-ui", 80, 80, "../../public/admin.html", logrus.New())
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/toggle", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = app.HandleToggle(c)
		require.Error(t, err)
		assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
	})

	t.Run("SetMaintenanceMode error", func(t *testing.T) {
		mockK8s := &MockK8sClient{setMaintError: fmt.Errorf("k8s error")}
		app, err := NewAdminApp(mockK8s, "route", "maintenance-page", "de-ui", 80, 80, "../../public/admin.html", logrus.New())
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/toggle", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = app.HandleToggle(c)
		require.Error(t, err)
		assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
	})
}

func TestHandleStatus(t *testing.T) {
	e := echo.New()

	t.Run("success", func(t *testing.T) {
		mockK8s := &MockK8sClient{isMaintenance: true}
		app, err := NewAdminApp(mockK8s, "route", "maintenance-page", "de-ui", 80, 80, "../../public/admin.html", logrus.New())
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/maintenance", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, app.HandleStatus(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.JSONEq(t, `{"maintenance": true}`, rec.Body.String())
		}
	})

	t.Run("error", func(t *testing.T) {
		mockK8s := &MockK8sClient{isMaintError: fmt.Errorf("k8s error")}
		app, err := NewAdminApp(mockK8s, "route", "maintenance-page", "de-ui", 80, 80, "../../public/admin.html", logrus.New())
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/maintenance", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = app.HandleStatus(c)
		require.Error(t, err)
		assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
	})
}

func TestHandleIndex(t *testing.T) {
	e := echo.New()

	t.Run("success", func(t *testing.T) {
		mockK8s := &MockK8sClient{isMaintenance: false}
		app, err := NewAdminApp(mockK8s, "route", "maintenance-page", "de-ui", 80, 80, "../../public/admin.html", logrus.New())
		require.NoError(t, err)
		app.Register(e)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if assert.NoError(t, app.HandleIndex(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), "Maintenance Mode Admin")
			assert.Contains(t, rec.Body.String(), "OFF (DE UI Active)")
		}
	})

	t.Run("error", func(t *testing.T) {
		mockK8s := &MockK8sClient{isMaintError: fmt.Errorf("k8s error")}
		app, err := NewAdminApp(mockK8s, "route", "maintenance-page", "de-ui", 80, 80, "../../public/admin.html", logrus.New())
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = app.HandleIndex(c)
		require.Error(t, err)
		assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
	})
}

func TestTemplate_Render(t *testing.T) {
	tmpl, err := template.New("test").Parse("{{.}}")
	require.NoError(t, err)

	renderer := &Template{templates: tmpl}
	rec := httptest.NewRecorder()
	err = renderer.Render(rec, "test", "hello", nil)
	assert.NoError(t, err)
	assert.Equal(t, "hello", rec.Body.String())
}

func TestNewAdminApp(t *testing.T) {
	mockK8s := &MockK8sClient{}
	
	t.Run("success", func(t *testing.T) {
		app, err := NewAdminApp(mockK8s, "route", "maint", "deui", 80, 80, "../../public/admin.html", logrus.New())
		assert.NoError(t, err)
		assert.NotNil(t, app)
	})

	t.Run("template error", func(t *testing.T) {
		app, err := NewAdminApp(mockK8s, "route", "maint", "deui", 80, 80, "non-existent.html", logrus.New())
		assert.Error(t, err)
		assert.Nil(t, app)
	})
}

