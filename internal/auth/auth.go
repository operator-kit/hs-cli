package auth

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const tokenURL = "https://api.helpscout.net/v2/oauth2/token"

// TokenSource returns an oauth2 token source that auto-fetches and refreshes.
func TokenSource(ctx context.Context, clientID, clientSecret string) oauth2.TokenSource {
	cfg := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
	}
	return cfg.TokenSource(ctx)
}

// HTTPClient returns an *http.Client that injects Bearer tokens automatically.
func HTTPClient(ctx context.Context, clientID, clientSecret string) *http.Client {
	cfg := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
	}
	return cfg.Client(ctx)
}
