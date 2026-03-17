package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nyx-nimbo/erebus-api/db"
	"go.mongodb.org/mongo-driver/bson"
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
		if req.Code == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Authorization code required", "code": 400})
		}

		cfg := *oauthConfig
		if req.RedirectURI != "" {
			cfg.RedirectURL = req.RedirectURI
		}

		token, err := cfg.Exchange(context.Background(), req.Code)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Failed to exchange code: " + err.Error(), "code": 401})
		}

		userInfo, err := fetchGoogleUser(token.AccessToken)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch user info", "code": 500})
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
