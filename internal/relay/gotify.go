package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"gotify-relay/internal/config"
)

type GotifyClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewGotifyClient(baseURL string, httpClient *http.Client) *GotifyClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &GotifyClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *GotifyClient) Push(ctx context.Context, memberName string, member config.Member, message Message) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode message for member %q: %w", memberName, err)
	}

	endpoint, err := url.Parse(c.baseURL + "/message")
	if err != nil {
		return fmt.Errorf("build gotify URL for member %q: %w", memberName, err)
	}
	query := endpoint.Query()
	query.Set("token", member.AppToken)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request for member %q: %w", memberName, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("push to member %q: %w", memberName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("push to member %q failed with status %d", memberName, resp.StatusCode)
	}

	return nil
}
