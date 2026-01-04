package hyper

import (
	"context"
	"errors"

	"github.com/charmbracelet/crush/internal/oauth"
)

// DeviceAuthResponse represents a device authorization response.
type DeviceAuthResponse struct {
	DeviceCode      string
	ExpiresIn       int64
	UserCode        string
	VerificationURL string
}

// IntrospectResponse represents a token introspection response.
type IntrospectResponse struct {
	Active bool
}

func InitiateDeviceAuth(context.Context) (*DeviceAuthResponse, error) {
	return nil, errors.New("hyper integration not available")
}

func PollForToken(context.Context, string, int64) (string, error) {
	return "", errors.New("hyper integration not available")
}

func ExchangeToken(context.Context, string) (*oauth.Token, error) {
	return nil, errors.New("hyper integration not available")
}

func IntrospectToken(context.Context, string) (*IntrospectResponse, error) {
	return nil, errors.New("hyper integration not available")
}
