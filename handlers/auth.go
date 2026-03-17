package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nyx-nimbo/erebus-api/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var oauthConfig *oauth2.Config

func InitOAuth(clientID, clientSecret string) {
	oauthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

type googleLoginRequest struct {
	Code        string `json:"code"`
	Credential  string `json:"credential"`
	RedirectURI string `json:"redirectUri"`
}

type googleUserInfo struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type jwtClaims struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	jwt.RegisteredClaims
}

func generateJWT(email, name, picture, secret string) (string, error) {
	claims := jwtClaims{
		Email:   email,
		Name:    name,
		Picture: picture,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GoogleLogin exchanges an OAuth code for user info and returns a JWT.
func GoogleLogin(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req googleLoginRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
		}

		var userInfo *googleUserInfo

		if req.Credential != "" {
			// Google Identity Services (One Tap / Sign-In button) sends a JWT credential
			info, err := verifyGoogleCredential(req.Credential)
			if err != nil {
				return c.Status(401).JSON(fiber.Map{"error": "Invalid credential: " + err.Error(), "code": 401})
			}
			userInfo = info
		} else if req.Code != "" {
			// Traditional OAuth2 authorization code flow
			cfg := *oauthConfig
			if req.RedirectURI != "" {
				cfg.RedirectURL = req.RedirectURI
			}

			token, err := cfg.Exchange(context.Background(), req.Code)
			if err != nil {
				return c.Status(401).JSON(fiber.Map{"error": "Failed to exchange code: " + err.Error(), "code": 401})
			}

			info, err := fetchGoogleUser(token.AccessToken)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch user info", "code": 500})
			}
			userInfo = info
		} else {
			return c.Status(400).JSON(fiber.Map{"error": "Authorization code or credential required", "code": 400})
		}

		upsertUser(userInfo)

		jwtToken, err := generateJWT(userInfo.Email, userInfo.Name, userInfo.Picture, jwtSecret)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to generate token", "code": 500})
		}

		return c.JSON(fiber.Map{
			"token": jwtToken,
			"user": fiber.Map{
				"email":   userInfo.Email,
				"name":    userInfo.Name,
				"picture": userInfo.Picture,
			},
		})
	}
}

// GetMe returns the current authenticated user's info from JWT.
func GetMe(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"email":   c.Locals("email"),
		"name":    c.Locals("name"),
		"picture": c.Locals("picture"),
	})
}

// RefreshToken issues a new JWT for the current user.
func RefreshToken(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		email, _ := c.Locals("email").(string)
		name, _ := c.Locals("name").(string)
		picture, _ := c.Locals("picture").(string)

		token, err := generateJWT(email, name, picture, jwtSecret)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to generate token", "code": 500})
		}
		return c.JSON(fiber.Map{"token": token})
	}
}

// verifyGoogleCredential validates a Google Identity Services JWT credential
// and extracts user info from it.
func verifyGoogleCredential(credential string) (*googleUserInfo, error) {
	// Verify the JWT by fetching Google's token info endpoint
	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + credential)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fiber.NewError(401, "Google token verification failed: "+string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var claims struct {
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
		Aud     string `json:"aud"`
	}
	if err := json.Unmarshal(body, &claims); err != nil {
		return nil, err
	}

	// Verify audience matches our client ID
	if oauthConfig != nil && claims.Aud != oauthConfig.ClientID {
		return nil, fiber.NewError(401, "Token audience mismatch")
	}

	return &googleUserInfo{
		Email:   claims.Email,
		Name:    claims.Name,
		Picture: claims.Picture,
	}, nil
}

func fetchGoogleUser(accessToken string) (*googleUserInfo, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func upsertUser(info *googleUserInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	coll := db.Collection("users")
	filter := bson.M{"email": info.Email}
	update := bson.M{
		"$set": bson.M{
			"name":      info.Name,
			"picture":   info.Picture,
			"updatedAt": time.Now(),
		},
		"$setOnInsert": bson.M{
			"email":     info.Email,
			"createdAt": time.Now(),
		},
	}

	upsert := true
	coll.UpdateOne(ctx, filter, update, &options.UpdateOptions{Upsert: &upsert})
}

// --- Google Services Connection (Gmail, Calendar, Drive) ---

var googleServicesScopes = []string{
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
	"https://www.googleapis.com/auth/gmail.modify",
	"https://www.googleapis.com/auth/gmail.send",
	"https://www.googleapis.com/auth/calendar",
	"https://www.googleapis.com/auth/drive",
}

// GoogleConnectURL returns a Google OAuth URL with Gmail/Calendar/Drive scopes.
func GoogleConnectURL(c *fiber.Ctx) error {
	if oauthConfig == nil {
		return c.Status(500).JSON(fiber.Map{"error": "OAuth not configured", "code": 500})
	}

	var req struct {
		RedirectURI string `json:"redirectUri"`
	}
	c.BodyParser(&req)

	// Build config with extended scopes
	cfg := &oauth2.Config{
		ClientID:     oauthConfig.ClientID,
		ClientSecret: oauthConfig.ClientSecret,
		Scopes:       googleServicesScopes,
		Endpoint:     google.Endpoint,
		RedirectURL:  req.RedirectURI,
	}

	// Generate a random state parameter
	stateBytes := make([]byte, 16)
	rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	return c.JSON(fiber.Map{
		"url":   url,
		"state": state,
	})
}

// GoogleCallback exchanges an auth code for tokens and stores them on the user.
func GoogleCallback(c *fiber.Ctx) error {
	if oauthConfig == nil {
		return c.Status(500).JSON(fiber.Map{"error": "OAuth not configured", "code": 500})
	}

	var req struct {
		Code        string `json:"code"`
		RedirectURI string `json:"redirectUri"`
	}
	if err := c.BodyParser(&req); err != nil || req.Code == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Authorization code required", "code": 400})
	}

	cfg := &oauth2.Config{
		ClientID:     oauthConfig.ClientID,
		ClientSecret: oauthConfig.ClientSecret,
		Scopes:       googleServicesScopes,
		Endpoint:     google.Endpoint,
		RedirectURL:  req.RedirectURI,
	}

	token, err := cfg.Exchange(context.Background(), req.Code)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Failed to exchange code: " + err.Error(), "code": 401})
	}

	// Get the authenticated user's email from JWT locals
	email, _ := c.Locals("email").(string)
	if email == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated", "code": 401})
	}

	// Store tokens on the user document
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"googleAccessToken":  token.AccessToken,
			"googleRefreshToken": token.RefreshToken,
			"googleTokenExpiry":  token.Expiry,
			"googleConnected":    true,
			"updatedAt":          time.Now(),
		},
	}

	result, err := db.Collection("users").UpdateOne(ctx, bson.M{"email": email}, update)
	if err != nil || result.MatchedCount == 0 {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to store tokens", "code": 500})
	}

	return c.JSON(fiber.Map{"connected": true})
}

// GoogleConnectionStatus checks whether the current user has connected Google Services.
func GoogleConnectionStatus(c *fiber.Ctx) error {
	email, _ := c.Locals("email").(string)
	if email == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated", "code": 401})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user bson.M
	err := db.Collection("users").FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return c.JSON(fiber.Map{"connected": false})
	}

	connected, _ := user["googleConnected"].(bool)
	return c.JSON(fiber.Map{"connected": connected})
}

// GoogleDisconnect removes stored Google tokens from the user.
func GoogleDisconnect(c *fiber.Ctx) error {
	email, _ := c.Locals("email").(string)
	if email == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated", "code": 401})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"googleConnected": false,
			"updatedAt":       time.Now(),
		},
		"$unset": bson.M{
			"googleAccessToken":  "",
			"googleRefreshToken": "",
			"googleTokenExpiry":  "",
		},
	}

	db.Collection("users").UpdateOne(ctx, bson.M{"email": email}, update)
	return c.JSON(fiber.Map{"disconnected": true})
}

// GetUserGoogleToken retrieves the stored Google access token for a user,
// refreshing it if expired.
func GetUserGoogleToken(email string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user bson.M
	err := db.Collection("users").FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return "", err
	}

	connected, _ := user["googleConnected"].(bool)
	if !connected {
		return "", fiber.NewError(403, "Google services not connected")
	}

	accessToken, _ := user["googleAccessToken"].(string)
	refreshToken, _ := user["googleRefreshToken"].(string)

	// Check if token is expired
	var expiry time.Time
	if exp, ok := user["googleTokenExpiry"].(primitive.DateTime); ok {
		expiry = exp.Time()
	}

	if time.Now().Before(expiry) && accessToken != "" {
		return accessToken, nil
	}

	// Token expired — refresh it
	if refreshToken == "" {
		return "", fiber.NewError(403, "Google token expired and no refresh token available")
	}

	cfg := &oauth2.Config{
		ClientID:     oauthConfig.ClientID,
		ClientSecret: oauthConfig.ClientSecret,
		Endpoint:     google.Endpoint,
	}

	tokenSource := cfg.TokenSource(context.Background(), &oauth2.Token{
		RefreshToken: refreshToken,
	})

	newToken, err := tokenSource.Token()
	if err != nil {
		// Mark as disconnected on refresh failure
		db.Collection("users").UpdateOne(ctx, bson.M{"email": email}, bson.M{
			"$set": bson.M{"googleConnected": false, "updatedAt": time.Now()},
		})
		return "", fiber.NewError(403, "Failed to refresh Google token: "+err.Error())
	}

	// Store refreshed token
	updateFields := bson.M{
		"googleAccessToken": newToken.AccessToken,
		"googleTokenExpiry": newToken.Expiry,
		"updatedAt":         time.Now(),
	}
	if newToken.RefreshToken != "" {
		updateFields["googleRefreshToken"] = newToken.RefreshToken
	}
	db.Collection("users").UpdateOne(ctx, bson.M{"email": email}, bson.M{"$set": updateFields})

	return newToken.AccessToken, nil
}
