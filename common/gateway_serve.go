package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

// defaultHTTPPort is the port a service's REST listener binds when HTTP_PORT is
// unset. Every service uses the same value because only one of them is ever
// published to the host - nginx is the only thing that needs to reach them, and
// it goes over the compose network.
const defaultHTTPPort = "8080"

// defaultHTTPShutdownTimeout bounds how long in-flight REST requests get to
// finish during shutdown.
const defaultHTTPShutdownTimeout = 5 * time.Second

// Gateway owns a service's REST listener: the ServeMux its handlers are
// registered on, and the http.Server fronting it.
//
// It exists so the five services that need one do not each carry the same forty
// lines of ServeMux construction, ListenAndServe and shutdown sequencing. The
// gRPC server deliberately stays in each main.go, where it already lives and
// where the per-service interceptor chain is set up.
//
// Every service's main.go pairs this with a loopback dial back into its own
// gRPC port - Register<Service>HandlerFromEndpoint, never HandlerServer, which
// would call the implementation in-process and skip every interceptor.
type Gateway struct {
	mux  *runtime.ServeMux
	http *http.Server
}

// NewGateway builds a service's REST handler.
//
// register is called with the finished mux and is where the service makes its
// Register<Service>HandlerFromEndpoint calls. extra options are passed through
// to GatewayMux, so a service can override the defaults.
func NewGateway(register func(*runtime.ServeMux) error, extra ...runtime.ServeMuxOption) (*Gateway, error) {
	mux := GatewayMux(extra...)
	if register != nil {
		if err := register(mux); err != nil {
			return nil, fmt.Errorf("registering REST handlers: %w", err)
		}
	}

	return &Gateway{
		mux: mux,
		http: &http.Server{
			Addr: ":" + HTTPPort(),
			// ReadHeaderTimeout bounds the slowloris window. The body itself is
			// left unbounded because assignment uploads are large; that route
			// applies its own limit with http.MaxBytesReader.
			ReadHeaderTimeout: 10 * time.Second,
			Handler:           mux,
		},
	}, nil
}

// HTTPPort resolves the REST listener's port from the environment, falling back
// to the shared default.
func HTTPPort() string {
	if port := os.Getenv("HTTP_PORT"); port != "" {
		return port
	}
	return defaultHTTPPort
}

// Handler exposes the mux, so a test can drive the REST surface over
// httptest.NewServer without binding a port.
func (g *Gateway) Handler() http.Handler { return g.mux }

// Addr reports the address the REST listener is bound to, for logging.
func (g *Gateway) Addr() string { return g.http.Addr }

// Serve starts the listener. It blocks until the listener stops, and returns
// nil for the normal http.ErrServerClosed that a shutdown produces.
func (g *Gateway) Serve() error {
	slog.Info("REST gateway running", "addr", g.http.Addr)
	if err := g.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown drains in-flight REST requests.
//
// It has to run *before* the gRPC server stops: every REST request is still
// waiting on a loopback gRPC call at this point, so tearing down gRPC first
// would fail them mid-translation.
func (g *Gateway) Shutdown(ctx context.Context) error {
	if err := g.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("draining the REST listener: %w", err)
	}
	return nil
}

// ShutdownWithTimeout is Shutdown bounded by defaultHTTPShutdownTimeout, for
// the common case of a service shutting down on a signal.
func (g *Gateway) ShutdownWithTimeout() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPShutdownTimeout)
	defer cancel()
	return g.Shutdown(ctx)
}
