package delivery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/egekocabas/kick-sim/internal/loopback"
)

const (
	HeaderMessageID      = "Kick-Event-Message-Id"
	HeaderSubscriptionID = "Kick-Event-Subscription-Id"
	HeaderSignature      = "Kick-Event-Signature"
	HeaderTimestamp      = "Kick-Event-Message-Timestamp"
	HeaderEventType      = "Kick-Event-Type"
	HeaderEventVersion   = "Kick-Event-Version"
	HeaderSimulator      = "Kick-Simulator"

	maxResponseBody = 64 * 1024
)

// Metadata contains the Kick webhook headers attached to a delivery.
type Metadata struct {
	MessageID      string
	SubscriptionID string
	Signature      string
	Timestamp      string
	EventType      string
	EventVersion   string
	OmitSignature  bool
}

// Result captures the observable HTTP response and delivery timing.
type Result struct {
	StatusCode    int
	Headers       http.Header
	Duration      time.Duration
	StartedAt     time.Time
	ResponseBody  []byte
	BodyTruncated bool
}

// Send posts one simulated webhook after enforcing the loopback destination policy.
func Send(ctx context.Context, client *http.Client, destination string, metadata Metadata, body []byte) (Result, error) {
	if err := loopback.ValidateURL(destination); err != nil {
		return Result{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, destination, bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("create webhook request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(HeaderMessageID, metadata.MessageID)
	request.Header.Set(HeaderSubscriptionID, metadata.SubscriptionID)
	if !metadata.OmitSignature {
		request.Header.Set(HeaderSignature, metadata.Signature)
	}
	request.Header.Set(HeaderTimestamp, metadata.Timestamp)
	request.Header.Set(HeaderEventType, metadata.EventType)
	request.Header.Set(HeaderEventVersion, metadata.EventVersion)
	request.Header.Set(HeaderSimulator, "kick-sim")

	if client == nil {
		client = NewLoopbackClient(10 * time.Second)
	}

	startedAt := time.Now()
	response, err := client.Do(request)
	result := Result{Duration: time.Since(startedAt), StartedAt: startedAt.UTC()}
	if err != nil {
		return result, fmt.Errorf("deliver webhook: %w", err)
	}
	defer response.Body.Close()

	result.StatusCode = response.StatusCode
	result.Headers = response.Header.Clone()
	result.ResponseBody, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBody+1))
	if err != nil {
		return result, fmt.Errorf("read webhook response: %w", err)
	}
	if len(result.ResponseBody) > maxResponseBody {
		result.ResponseBody = result.ResponseBody[:maxResponseBody]
		result.BodyTruncated = true
	}
	return result, nil
}

// NewLoopbackClient returns an HTTP client that disables proxies and redirects
// and verifies every resolved destination address is loopback before dialing.
func NewLoopbackClient(timeout time.Duration) *http.Client {
	return newLoopbackClient(timeout, net.DefaultResolver, &net.Dialer{})
}

type addressResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type contextDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

func newLoopbackClient(timeout time.Duration, resolver addressResolver, dialer contextDialer) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("parse destination address: %w", err)
		}
		addresses, err := resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve destination host: %w", err)
		}
		if len(addresses) == 0 {
			return nil, errors.New("destination host resolved to no addresses")
		}
		// Reject the entire DNS answer if any address escapes loopback. This avoids
		// address-order-dependent behavior and DNS rebinding through mixed answers.
		for _, address := range addresses {
			if !address.IsLoopback() {
				return nil, fmt.Errorf("destination host resolved outside loopback: %s", address)
			}
		}

		var dialError error
		for _, address := range addresses {
			connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.String(), port))
			if err == nil {
				return connection, nil
			}
			dialError = err
		}
		return nil, fmt.Errorf("connect to loopback destination: %w", dialError)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("webhook redirects are disabled")
		},
	}
}
