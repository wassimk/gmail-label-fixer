package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const tokenFile = "token.json"

func GetGmailService() (*gmail.Service, error) {
	ctx := context.Background()

	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v.\n\nPlease ensure you have:\n1. Created OAuth 2.0 credentials in Google Cloud Console\n2. Downloaded the credentials JSON file\n3. Renamed it to 'credentials.json' in the current directory", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailModifyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	// Set redirect URI to localhost for desktop applications
	config.RedirectURL = "http://localhost:8080"

	client := getClient(config)

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Gmail client: %v", err)
	}

	return srv, nil
}

func getClient(config *oauth2.Config) *http.Client {
	tokFile := tokenFile
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	// Create channels for communication
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Start local server
	server := &http.Server{Addr: ":8080"}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			if errMsg != "" {
				errCh <- fmt.Errorf("authorization error: %s", errMsg)
			} else {
				errCh <- fmt.Errorf("no authorization code received")
			}
			http.Error(w, "Authorization failed", 400)
			return
		}

		// Send success response
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
			<html>
			<head>
				<meta charset="UTF-8">
				<title>Gmail Label Fixer - Success</title>
			</head>
			<body style="font-family: Arial, sans-serif; text-align: center; margin-top: 50px;">
				<h2>🎉 Authentication Successful!</h2>
				<p>You can close this browser tab and return to the terminal.</p>
				<script>setTimeout(() => window.close(), 3000);</script>
			</body>
			</html>
		`)

		codeCh <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server error: %v", err)
		}
	}()

	// Generate auth URL and display instructions
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("\n🔐 Gmail Authentication Required\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Opening your browser for authentication...\n")
	fmt.Printf("If it doesn't open automatically, visit:\n")
	fmt.Printf("   %v\n", authURL)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Try to open browser (will fail gracefully if not possible)
	openBrowser(authURL)

	// Wait for response
	var code string
	select {
	case code = <-codeCh:
		fmt.Printf("✅ Authorization received!\n")
	case err := <-errCh:
		server.Close()
		log.Fatalf("Authorization failed: %v", err)
	}

	// Cleanup server
	server.Close()

	// Exchange code for token
	fmt.Printf("🔄 Exchanging authorization code for access token...\n")
	tok, err := config.Exchange(context.TODO(), code)
	if err != nil {
		log.Fatalf("Unable to retrieve token: %v", err)
	}

	fmt.Printf("✅ Authentication successful!\n\n")
	return tok
}

func openBrowser(url string) {
	// Try to open browser on different platforms
	var cmd string
	var args []string

	switch {
	case commandExists("open"): // macOS
		cmd = "open"
		args = []string{url}
	case commandExists("xdg-open"): // Linux
		cmd = "xdg-open"
		args = []string{url}
	case commandExists("cmd"): // Windows
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return // Can't open browser
	}

	// Execute command (ignore errors)
	go func() {
		exec.Command(cmd, args...).Start()
	}()
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
