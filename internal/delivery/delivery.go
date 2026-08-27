package delivery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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

type Metadata struct {
	MessageID      string
	SubscriptionID string
	Signature      string
	Timestamp      string
	EventType      string
	EventVersion   string
	OmitSignature  bool
}

type Result struct {
	StatusCode    int
	Headers       http.Header
	Duration      time.Duration
	StartedAt     time.Time
	ResponseBody  []byte
	BodyTruncated bool
}

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

func NewLoopbackClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("parse destination address: %w", err)
		}
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve destination host: %w", err)
		}
		if len(addresses) == 0 {
			return nil, errors.New("destination host resolved to no addresses")
		}
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
