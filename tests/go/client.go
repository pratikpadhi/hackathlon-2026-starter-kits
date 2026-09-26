package sdet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Client struct {
	API  string
	Auth string
	http *http.Client
}

func NewClient() *Client {
	return &Client{
		API:  getenv("API_URL", "http://127.0.0.1:8081"),
		Auth: getenv("AUTHORITY_URL", "http://127.0.0.1:9000"),
		http: &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *Client) Reset() error {
	if _, _, err := c.do(http.MethodPost, c.Auth+"/admin/reset", map[string]string{}); err != nil {
		return err
	}
	_, _, err := c.do(http.MethodPost, c.API+"/admin/reset", map[string]string{})
	return err
}

func (c *Client) SetMode(mode string) error {
	_, _, err := c.do(http.MethodPost, c.Auth+"/admin/mode", map[string]string{"mode": mode})
	return err
}

func (c *Client) Reserve(user, item, key string, qty int) (int, map[string]any, error) {
	body, _ := json.Marshal(map[string]any{"itemId": item, "userId": user, "qty": qty})
	req, err := http.NewRequest(http.MethodPost, c.API+"/reservations", bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return resp.StatusCode, out, nil
}

func (c *Client) Health() (map[string]any, error) {
	_, b, err := c.do(http.MethodGet, c.API+"/health", nil)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out, nil
}

func (c *Client) List(user string) ([]map[string]any, error) {
	_, b, err := c.do(http.MethodGet, c.API+"/reservations?userId="+user, nil)
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("list: %s", string(b))
	}
	return out, nil
}

func (c *Client) do(method, url string, body any) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
