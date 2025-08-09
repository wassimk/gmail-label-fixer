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

const (
	tokenFile = "token.json"
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

	// Use out-of-band flow for CLI applications - more reliable than device code
	config.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"

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
	// Generate authorization URL with out-of-band redirect
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	
	fmt.Printf("\n🔐 Gmail Authentication Required\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📱 Please complete the authorization in your browser:\n")
	fmt.Printf("   1. Visit: %s\n", authURL)
	fmt.Printf("   2. Complete the authorization process\n")
	fmt.Printf("   3. Copy the authorization code when prompted\n")
	fmt.Printf("\n💡 Tip: The URL will be opened automatically if possible\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Try to open browser automatically
	openBrowser(authURL)

	// Prompt user to enter the authorization code
	fmt.Printf("\n📋 Enter the authorization code: ")
	var authCode string
	fmt.Scanln(&authCode)
	
	if authCode == "" {
		log.Fatal("No authorization code provided")
	}
	
	// Exchange the authorization code for a token
	fmt.Printf("🔄 Exchanging authorization code for access token...\n")
	token, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token: %v", err)
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
