package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type DexConfig struct {
	Issuer       string   `json:"issuer" yaml:"issuer"`               // Backend connection URL
	PublicIssuer string   `json:"public_issuer" yaml:"public_issuer"` // Frontend OAuth URL
	ClientID     string   `json:"client_id" yaml:"client_id"`
	ClientSecret string   `json:"client_secret" yaml:"client_secret"`
	RedirectURL  string   `json:"redirect_url" yaml:"redirect_url"`
	Scopes       []string `json:"scopes" yaml:"scopes"`
}

type DexAuthProvider struct {
	config       *DexConfig
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

type AuthClaims struct {
	jwt.RegisteredClaims
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Name              string   `json:"name"`
	Groups            []string `json:"groups"`
	PreferredUsername string   `json:"preferred_username"`
	ConnectorID       string   `json:"connector_id"`       // Dex connector ID (github, google, microsoft)
	ConnectorData     map[string]interface{} `json:"connector_data"` // Additional connector-specific data
}

type Session struct {
	ID           string    `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	Groups       []string  `json:"groups"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func NewDexAuthProvider(config *DexConfig) (*DexAuthProvider, error) {
	ctx := context.Background()

	// When running in k8s, config.Issuer is the internal service URL for pod-to-pod
	// communication, while config.PublicIssuer is the external URL that Dex advertises
	// as its issuer. Use InsecureIssuerURLContext to connect via internal URL while
	// accepting the external issuer in OIDC discovery.
	issuerURL := config.Issuer
	if config.PublicIssuer != "" && config.PublicIssuer != config.Issuer {
		ctx = oidc.InsecureIssuerURLContext(ctx, config.Issuer)
		issuerURL = config.PublicIssuer
	}

	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	// Use discovery endpoints but override token URL to use internal service URL
	endpoint := provider.Endpoint()
	if config.PublicIssuer != "" && config.PublicIssuer != config.Issuer {
		endpoint.TokenURL = strings.TrimSuffix(config.Issuer, "/") + "/token"
	}

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint:     endpoint,
		Scopes:       config.Scopes,
	}

	if len(oauth2Config.Scopes) == 0 {
		oauth2Config.Scopes = []string{oidc.ScopeOpenID, "profile", "email", "groups"}
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: config.ClientID,
	})

	return &DexAuthProvider{
		config:       config,
		provider:     provider,
		oauth2Config: oauth2Config,
		verifier:     verifier,
	}, nil
}

func (d *DexAuthProvider) GetAuthURL(state string) string {
	return d.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (d *DexAuthProvider) GenerateState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (d *DexAuthProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return d.oauth2Config.Exchange(ctx, code)
}

func (d *DexAuthProvider) VerifyIDToken(ctx context.Context, rawIDToken string) (*AuthClaims, error) {
	idToken, err := d.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	var claims AuthClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	return &claims, nil
}

func (d *DexAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	tokenSource := d.oauth2Config.TokenSource(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
	})

	return tokenSource.Token()
}

func (d *DexAuthProvider) GetUserInfo(ctx context.Context, accessToken string) (map[string]interface{}, error) {
	userInfoURL := strings.TrimSuffix(d.config.Issuer, "/") + "/userinfo"

	req, err := http.NewRequestWithContext(ctx, "GET", userInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed with status: %d", resp.StatusCode)
	}

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return userInfo, nil
}

func (d *DexAuthProvider) RevokeToken(ctx context.Context, token string) error {
	revokeURL := strings.TrimSuffix(d.config.Issuer, "/") + "/token/revoke"

	req, err := http.NewRequestWithContext(ctx, "POST", revokeURL, strings.NewReader("token="+token))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(d.config.ClientID, d.config.ClientSecret)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("revoke request failed with status: %d", resp.StatusCode)
	}

	return nil
}
