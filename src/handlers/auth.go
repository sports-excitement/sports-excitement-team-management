package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"golang.org/x/crypto/bcrypt"

	"sports-excitement-team-management/src/config"
	"sports-excitement-team-management/src/database"
)

var store *session.Store

// initSession initializes the session store
func initSession() {
	store = session.New(session.Config{
		KeyLookup:      "cookie:session_id",
		CookieDomain:   "",
		CookiePath:     "/",
		CookieSecure:   false,
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		Expiration:     24 * time.Hour,
	})
}

// TurnstileResponse represents the response from Cloudflare Turnstile API
type TurnstileResponse struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
}

// ShowLogin displays the login page
func ShowLogin(c *fiber.Ctx) error {
	sess, err := store.Get(c)
	if err != nil {
		return err
	}
	if sess.Get("authenticated") == true {
		return c.Redirect("/dashboard")
	}

	return c.Render("auth/login", fiber.Map{
		"Title":           "Admin Login",
		"TurnstileSiteKey": config.AppConfig.TurnstileSiteKey,
		"Error":           sess.Get("error"),
	})
}

// HandleLogin processes the login form
func HandleLogin(c *fiber.Ctx) error {
	sess, err := store.Get(c)
	if err != nil {
		return err
	}
	defer sess.Save()

	username := c.FormValue("username")
	password := c.FormValue("password")
	turnstileToken := c.FormValue("cf-turnstile-response")

	// Validate required fields
	if username == "" || password == "" {
		sess.Set("error", "Username and password are required")
		return c.Redirect("/login")
	}

	// Verify Turnstile token
	if !verifyTurnstile(turnstileToken, c.IP()) {
		sess.Set("error", "Security verification failed. Please try again.")
		return c.Redirect("/login")
	}

	// Get admin user from database
	var admin database.Admin
	result := database.DB.Where("username = ?", username).First(&admin)
	if result.Error != nil {
		sess.Set("error", "Invalid credentials")
		return c.Redirect("/login")
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(password))
	if err != nil {
		sess.Set("error", "Invalid credentials")
		return c.Redirect("/login")
	}

	// Set session
	sess.Set("authenticated", true)
	sess.Set("user_id", admin.ID)
	sess.Set("username", admin.Username)

	return c.Redirect("/dashboard")
}

// HandleLogout processes logout
func HandleLogout(c *fiber.Ctx) error {
	sess, err := store.Get(c)
	if err != nil {
		return err
	}
	sess.Destroy()
	return c.Redirect("/login")
}

// verifyTurnstile verifies the Turnstile token with Cloudflare
func verifyTurnstile(token, remoteIP string) bool {
	if config.AppConfig.TurnstileSecretKey == "" {
		// Skip verification if secret key is not configured
		return true
	}

	data := map[string]string{
		"secret":   config.AppConfig.TurnstileSecretKey,
		"response": token,
		"remoteip": remoteIP,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return false
	}

	resp, err := http.Post(
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var turnstileResp TurnstileResponse
	err = json.Unmarshal(body, &turnstileResp)
	if err != nil {
		return false
	}

	return turnstileResp.Success
}

// AuthMiddleware checks if the user is authenticated
func AuthMiddleware(c *fiber.Ctx) error {
	if store == nil {
		initSession()
	}

	sess, err := store.Get(c)
	if err != nil {
		return err
	}
	
	if sess.Get("authenticated") != true {
		if c.Path() == "/login" || c.Path() == "/" {
			return c.Next()
		}
		return c.Redirect("/login")
	}

	// Store user info in context
	c.Locals("user_id", sess.Get("user_id"))
	c.Locals("username", sess.Get("username"))

	return c.Next()
}

// RequireAuth middleware that redirects unauthenticated users
func RequireAuth(c *fiber.Ctx) error {
	if store == nil {
		initSession()
	}

	sess, err := store.Get(c)
	if err != nil {
		return err
	}
	
	if sess.Get("authenticated") != true {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	c.Locals("user_id", sess.Get("user_id"))
	c.Locals("username", sess.Get("username"))

	return c.Next()
} 