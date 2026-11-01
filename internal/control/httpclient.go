package control

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AllowInsecureHTTPForTest permits http:// API bases in unit tests only.
var AllowInsecureHTTPForTest bool

// NewSecureAPIClient creates a client with TLS redirect protections.
func NewSecureAPIClient(baseURL string) (*APIClient, error) {
	u := strings.TrimRight(baseURL, "/")
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "https" {
		if !AllowInsecureHTTPForTest || parsed.Scheme != "http" {
			return nil, fmt.Errorf("coordinator URL must use https")
		}
	}
	return &APIClient{
		BaseURL: u,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				if len(via) >= 1 && req.URL.Host != via[0].URL.Host {
					return fmt.Errorf("redirect to untrusted host blocked")
				}
				if req.URL.Scheme != "https" && !(AllowInsecureHTTPForTest && req.URL.Scheme == "http") {
					return fmt.Errorf("redirect to non-https blocked")
				}
				return nil
			},
		},
	}, nil
}

// APIError carries HTTP status for callers (no secret fields).
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s (http %d)", e.Message, e.Status)
	}
	return fmt.Sprintf("http %d", e.Status)
}

func IsHTTPStatus(err error, code int) bool {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Status == code
	}
	return false
}
