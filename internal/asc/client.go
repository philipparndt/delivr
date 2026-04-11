// Package asc provides a minimal App Store Connect API client
// for uploading metadata and screenshots.
package asc

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/md5"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const baseURL = "https://api.appstoreconnect.apple.com/v1"

// Client is a minimal App Store Connect API client.
type Client struct {
	keyID    string
	issuerID string
	key      *ecdsa.PrivateKey
	http     *http.Client
	Verbose  bool
}

// NewClient creates a new ASC API client from the given credentials.
// keySource is either a file path to a .p8 key or the PEM content directly.
func NewClient(keyID, issuerID, keySource string) (*Client, error) {
	key, err := parsePrivateKey(keySource)
	if err != nil {
		return nil, fmt.Errorf("load API key: %w", err)
	}
	return &Client{
		keyID:    keyID,
		issuerID: issuerID,
		key:      key,
		http:     &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func parsePrivateKey(source string) (*ecdsa.PrivateKey, error) {
	var data []byte
	if strings.HasPrefix(strings.TrimSpace(source), "-----BEGIN") {
		// Inline PEM content
		data = []byte(source)
	} else {
		// File path
		var err error
		data, err = os.ReadFile(source)
		if err != nil {
			return nil, err
		}
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}
	ecKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not ECDSA P-256")
	}
	return ecKey, nil
}

func (c *Client) createToken() (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": c.issuerID,
		"iat": now.Unix(),
		"exp": now.Add(20 * time.Minute).Unix(),
		"aud": "appstoreconnect-v1",
	})
	token.Header["kid"] = c.keyID
	return token.SignedString(c.key)
}

func (c *Client) request(method, url string, body interface{}) ([]byte, int, error) {
	token, err := c.createToken()
	if err != nil {
		return nil, 0, fmt.Errorf("create token: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.Verbose {
		fmt.Printf("  %s %s\n", method, url)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("API %d: %s", resp.StatusCode, truncate(string(respBody), 2000))
	}
	return respBody, resp.StatusCode, nil
}

func (c *Client) get(path string) ([]byte, error) {
	data, _, err := c.request("GET", baseURL+path, nil)
	return data, err
}

func (c *Client) patch(path string, body interface{}) error {
	_, _, err := c.request("PATCH", baseURL+path, body)
	return err
}

func (c *Client) post(path string, body interface{}) ([]byte, error) {
	data, _, err := c.request("POST", baseURL+path, body)
	return data, err
}

func (c *Client) del(path string) error {
	_, _, err := c.request("DELETE", baseURL+path, nil)
	return err
}

// uploadRaw uploads raw bytes to a URL with custom headers (for screenshot upload operations).
func (c *Client) uploadRaw(method, url string, data []byte, headers map[string]string) error {
	req, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if c.Verbose {
		fmt.Printf("  %s %s (%d bytes)\n", method, url, len(data))
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload %d: %s", resp.StatusCode, truncate(string(body), 300))
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}

func fileMD5(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:]), nil
}
