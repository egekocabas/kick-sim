package studio

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/egekocabas/kick-sim/internal/app"
	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
	webassets "github.com/egekocabas/kick-sim/web"
	"github.com/go-chi/chi/v5"
)

const (
	DefaultAddress = "127.0.0.1:4321"
	controlCookie  = "kick_sim_control"
)

type Options struct {
	Address     string
	OpenBrowser bool
	Output      io.Writer
}

type RunningServer struct {
	URL      string
	Embedded bool
	Token    string
	Handler  http.Handler
}

func Run(ctx context.Context, service *app.Service, options Options) error {
	address := options.Address
	if address == "" {
		address = DefaultAddress
	}
	listener, err := listen(address)
	if err != nil {
		return err
	}
	defer listener.Close()
	running, err := New(service, listener.Addr().String())
	if err != nil {
		return err
	}
	server := &http.Server{
		Handler:           running.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 * 1024,
	}
	output := options.Output
	if output == nil {
		output = io.Discard
	}
	fmt.Fprintf(output, "Kick Sim Studio: %s\n", running.URL)
	if !running.Embedded {
		fmt.Fprintln(output, "Warning: this development build is serving the Studio asset stub")
	}
	if options.OpenBrowser {
		if err := openBrowser(running.URL); err != nil {
			fmt.Fprintf(output, "Open the Studio manually: %s (%v)\n", running.URL, err)
		}
	}

	errorsChannel := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errorsChannel <- err
	}()
	select {
	case err := <-errorsChannel:
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shut down Studio: %w", err)
		}
		return <-errorsChannel
	}
}

func New(service *app.Service, address string) (RunningServer, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return RunningServer{}, fmt.Errorf("parse Studio listener: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return RunningServer{}, errors.New("Studio listener must use a loopback IP address")
	}
	if port == "" {
		return RunningServer{}, errors.New("Studio listener requires a port")
	}
	token, err := newControlToken()
	if err != nil {
		return RunningServer{}, err
	}
	origin := "http://" + net.JoinHostPort(host, port)
	router := chi.NewRouter()
	kickopenapi.New(router, NewBackend(service))
	assets, embedded := webassets.Assets()
	staticHandler := http.FileServer(http.FS(assets))
	router.NotFound(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			http.NotFound(writer, request)
			return
		}
		if _, err := fs.Stat(assets, strings.TrimPrefix(request.URL.Path, "/")); err != nil {
			request.URL.Path = "/"
		}
		staticHandler.ServeHTTP(writer, request)
	})
	handler := securityMiddleware(origin, net.JoinHostPort(host, port), token, router)
	return RunningServer{URL: origin, Embedded: embedded, Token: token, Handler: handler}, nil
}

func listen(address string) (net.Listener, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("parse Studio address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, errors.New("Studio address must use a loopback IP address")
	}
	listener, err := net.Listen("tcp", address)
	if err == nil {
		return listener, nil
	}
	if address == DefaultAddress && errors.Is(err, syscall.EADDRINUSE) {
		listener, fallbackErr := net.Listen("tcp", "127.0.0.1:0")
		if fallbackErr == nil {
			return listener, nil
		}
		return nil, fmt.Errorf("listen on preferred and fallback Studio ports: %w", fallbackErr)
	}
	return nil, fmt.Errorf("listen for Studio: %w", err)
}

func securityMiddleware(origin, allowedHost, token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		if request.Host != allowedHost {
			http.Error(writer, "invalid Studio host", http.StatusForbidden)
			return
		}
		if request.Method == http.MethodGet && !strings.HasPrefix(request.URL.Path, "/api/") {
			http.SetCookie(writer, &http.Cookie{
				Name: controlCookie, Value: token, Path: "/api", HttpOnly: true,
				SameSite: http.SameSiteStrictMode, Secure: false,
			})
		}
		if isMutating(request.Method) {
			if !strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "application/json") {
				http.Error(writer, "mutating Studio requests require application/json", http.StatusUnsupportedMediaType)
				return
			}
			requestOrigin := request.Header.Get("Origin")
			if requestOrigin != "" && requestOrigin != origin {
				http.Error(writer, "invalid Studio origin", http.StatusForbidden)
				return
			}
			if !hasControlToken(request, token) {
				http.Error(writer, "missing Studio control token", http.StatusUnauthorized)
				return
			}
			if requestOrigin == "" && !hasBearerToken(request, token) {
				http.Error(writer, "non-browser Studio requests require bearer authentication", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}

func hasControlToken(request *http.Request, token string) bool {
	if hasBearerToken(request, token) {
		return true
	}
	cookie, err := request.Cookie(controlCookie)
	return err == nil && secureEqual(cookie.Value, token)
}

func hasBearerToken(request *http.Request, token string) bool {
	prefix, value, found := strings.Cut(request.Header.Get("Authorization"), " ")
	return found && strings.EqualFold(prefix, "Bearer") && secureEqual(value, token)
}

func secureEqual(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func isMutating(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}

func newControlToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate Studio control token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
