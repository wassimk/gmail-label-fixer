package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	tokenFile = "token.json"
	// Loopback configuration for secure CLI OAuth flow
	loopbackHost = "127.0.0.1"
)

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

	// Use loopback flow for CLI applications (Google's recommended approach)
	// The redirect URI will be set dynamically to an available port

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
	// Find an available port for the loopback server
	listener, err := net.Listen("tcp", loopbackHost+":0")
	if err != nil {
		log.Fatalf("Unable to create loopback server: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Set loopback redirect URI (Google's recommended approach for CLI apps)
	redirectURL := fmt.Sprintf("http://%s:%d/callback", loopbackHost, port)
	config.RedirectURL = redirectURL

	// Create channels for communication between server and main goroutine
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Create HTTP server to capture the authorization code
	server := &http.Server{
		Addr: fmt.Sprintf("%s:%d", loopbackHost, port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract authorization code from callback
			code := r.URL.Query().Get("code")
			if code == "" {
				errMsg := r.URL.Query().Get("error")
				if errMsg != "" {
					errCh <- fmt.Errorf("authorization error: %s", errMsg)
				} else {
					errCh <- fmt.Errorf("no authorization code received")
				}
				http.Error(w, "Authorization failed", http.StatusBadRequest)
				return
			}

			// Send success response to user
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Gmail Label Fixer - Authentication Successful</title>
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
		       text-align: center; padding: 50px; background: #f8f9fa; }
		.success { color: #28a745; font-size: 24px; margin-bottom: 20px; }
		.message { color: #495057; font-size: 16px; }
	</style>
</head>
<body>
	<div class="success">🎉 Authentication Successful!</div>
	<div class="message">You can close this browser tab and return to the terminal.</div>
	<script>
		// Auto-close tab after 3 seconds
		setTimeout(() => { 
			try { window.close(); } catch(e) { /* ignore */ }
		}, 3000);
	</script>
</body>
</html>`)

			codeCh <- code
		}),
	}

	// Start the server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("loopback server error: %v", err)
		}
	}()

	// Generate authorization URL
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	fmt.Printf("\n🔐 Gmail Authentication Required\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("🌐 Opening browser for secure authentication...\n")
	fmt.Printf("   URL: %s\n", authURL)
	fmt.Printf("\n💡 This will open your browser and redirect back to this application\n")
	fmt.Printf("   securely. No manual code copying required!\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Open browser automatically
	openBrowser(authURL)

	// Wait for authorization response or timeout
	var code string
	select {
	case code = <-codeCh:
		fmt.Printf("✅ Authorization received!\n")
	case err := <-errCh:
		server.Shutdown(context.Background())
		log.Fatalf("Authorization failed: %v", err)
	case <-time.After(5 * time.Minute):
		server.Shutdown(context.Background())
		log.Fatal("Authorization timed out after 5 minutes")
	}

	// Shutdown server gracefully
	server.Shutdown(context.Background())

	// Exchange authorization code for token
	fmt.Printf("🔄 Exchanging authorization code for access token...\n")
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		log.Fatalf("Unable to retrieve token: %v\n\n💡 Make sure your OAuth client is configured as 'Desktop application':\n   https://console.cloud.google.com/apis/credentials", err)
	}

	fmt.Printf("✅ Authentication successful!\n\n")
	return token
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

	// Remove existing file first to ensure proper permissions
	os.Remove(path)

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(token); err != nil {
		log.Fatalf("Unable to encode oauth token: %v", err)
	}
}
