package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

type Client struct {
	baseURL string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{},
	}
}

func (c *Client) SendContent(content *types.ClipboardContent) error {
	url := fmt.Sprintf("%s/clipboard", c.baseURL)

	payload, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal clipboard content: %v", err)
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to send clipboard content: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("backend returned non-OK status: %s", resp.Status)
	}

	return nil
}