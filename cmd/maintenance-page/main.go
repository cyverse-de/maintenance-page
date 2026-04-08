package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cyverse-de/maintenance-page/internal/k8s"
	"github.com/cyverse-de/maintenance-page/internal/server"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func setupEcho(log *logrus.Logger) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			log.WithFields(logrus.Fields{
				"URI":    v.URI,
				"status": v.Status,
			}).Info("request")
			return nil
		},
	}))
	e.Use(middleware.Recover())
	return e
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	var (
		maintenancePageService = flag.String("maintenance-page-service", getEnv("MAINTENANCE_PAGE_SERVICE", "maintenance-page"), "The name of the K8s Service for the loading page.")
		adminPageService       = flag.String("admin-page-service", getEnv("ADMIN_PAGE_SERVICE", "maintenance-page-admin"), "The name of the K8s Service for the admin page.")
		basicAuthUsername      = flag.String("basic-auth-username", getEnv("BASIC_AUTH_USERNAME", ""), "The username for the admin page.")
		basicAuthPassword      = flag.String("basic-auth-password", getEnv("BASIC_AUTH_PASSWORD", ""), "The password for the admin page.")
		port                   = flag.Int("port", 8080, "The port to listen on for the maintenance page.")
		adminPort              = flag.Int("admin-port", 8081, "The port to listen on for the admin page.")
		kubeconfig             = flag.String("kubeconfig", getEnv("KUBECONFIG", ""), "Path to kubeconfig (empty for in-cluster)")
		namespace              = flag.String("namespace", getEnv("NAMESPACE", "prod"), "The namespace to operate in.")
		httpRouteName          = flag.String("sonora-route-name", getEnv("SONORA_ROUTE_NAME", "discoenv-routes"), "The name of the HTTPRoute to toggle.")
		deUIService            = flag.String("sonora-service", getEnv("SONORA_SERVICE", "sonora"), "The name of the normal DE UI service.")
		deUIPort               = flag.Int("sonora-port", 80, "The port of the normal DE UI service.")
		adminTemplate          = flag.String("admin-template", getEnv("ADMIN_TEMPLATE", "public/admin.html"), "The path to the admin page template.")
	)
	flag.Parse()

	if *basicAuthUsername == "" || *basicAuthPassword == "" {
		log.Fatal("--basic-auth-username and --basic-auth-password (or corresponding environment variables) are required")
	}

	// Initialize K8s client
	k8sClient, err := k8s.NewClient(*kubeconfig, *namespace, log)
	if err != nil {
		log.Fatalf("failed to initialize k8s client: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Labels for the services
	labels := map[string]string{"app": "maintenance-page"}

	// Ensure Maintenance Page Service
	ensureCtx, ensureCancel := context.WithTimeout(ctx, 30*time.Second)
	defer ensureCancel()

	if err := k8sClient.EnsureService(ensureCtx, *maintenancePageService, 80, int32(*port), labels); err != nil {
		log.Errorf("failed to ensure maintenance page service: %v", err)
	}

	// Ensure Admin Page Service
	if err := k8sClient.EnsureService(ensureCtx, *adminPageService, 80, int32(*adminPort), labels); err != nil {
		log.Errorf("failed to ensure admin page service: %v", err)
	}

	// Setup Maintenance Page Server
	maintenanceEcho := setupEcho(log)
	maintenanceEcho.Static("/", "public")

	// Setup Admin Page Server
	adminEcho := setupEcho(log)
	adminEcho.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
		return username == *basicAuthUsername && password == *basicAuthPassword, nil
	}))

	adminApp, err := server.NewAdminApp(k8sClient, *httpRouteName, *maintenancePageService, *deUIService, 80, int32(*deUIPort), *adminTemplate, log)
	if err != nil {
		log.Fatalf("failed to initialize admin app: %v", err)
	}
	adminApp.Register(adminEcho)

	// Start servers
	go func() {
		addr := fmt.Sprintf(":%d", *port)
		log.Infof("Starting maintenance page server on %s", addr)
		if err := maintenanceEcho.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Errorf("maintenance page server failed: %v", err)
		}
	}()

	go func() {
		addr := fmt.Sprintf(":%d", *adminPort)
		log.Infof("Starting admin page server on %s", addr)
		if err := adminEcho.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Errorf("admin page server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	log.Info("Shutting down servers...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := maintenanceEcho.Shutdown(shutdownCtx); err != nil {
		log.Errorf("maintenance page server shutdown error: %v", err)
	}
	if err := adminEcho.Shutdown(shutdownCtx); err != nil {
		log.Errorf("admin page server shutdown error: %v", err)
	}
}
