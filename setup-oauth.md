# Gmail API & OAuth Setup Guide

Complete setup guide for Gmail Label Fixer authentication. Follow these steps to enable Gmail API access and configure OAuth credentials.

## Prerequisites
- A Google account
- Access to [Google Cloud Console](https://console.cloud.google.com/)
- The `gmail-label-fixer` executable

## Step 1: Configure OAuth Client in Google Cloud Console

1. **Go to Google Cloud Console**: https://console.cloud.google.com/
2. **Select your project** (or create a new one)
3. **Enable Gmail API**:
   - Go to "APIs & Services" → "Library"
   - Search for "Gmail API"
   - Click "Enable"

4. **Create OAuth 2.0 Credentials**:
   - Go to "APIs & Services" → "Credentials"
   - Click "Create Credentials" → "OAuth client ID"
   
5. **Configure the OAuth client**:
   - **Application type**: Desktop application
   - **Name**: Gmail Label Fixer
   - **Authorized redirect URIs**: No redirect URIs needed for desktop apps!
     (The app will use dynamic localhost ports automatically)
   
6. **Download credentials**:
   - Click "Download" to get the JSON file
   - Rename it to `credentials.json`
   - Place it in the same directory as the `gmail-label-fixer` executable

## Step 2: OAuth Consent Screen Setup

If this is your first OAuth client, you may need to configure the consent screen:

1. **Go to "OAuth consent screen"** in Google Cloud Console
2. **Choose "External"** (unless you're in a Google Workspace org)
3. **Fill required fields**:
   - App name: Gmail Label Fixer
   - User support email: your email
   - Developer contact information: your email
4. **Add scopes**: Add the Gmail API scope if prompted
5. **Save and continue**

## Step 3: Test Users (if needed)

If your app is in "Testing" mode:
1. Go to "Test users" section
2. Add your Gmail address as a test user
3. Save

## Step 4: Verify Your credentials.json

Your `credentials.json` should look similar to:
```json
{
  "installed": {
    "client_id": "your-client-id.googleusercontent.com",
    "project_id": "your-project-id",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "client_secret": "your-client-secret",
    "redirect_uris": ["http://localhost"]
  }
}
```

## Step 5: Run the Tool Again

```bash
./gmail-label-fixer analyze
```

## Troubleshooting

### Error: "invalid_request"
- Make sure your OAuth client is configured as "Desktop application"
- No redirect URIs should be needed for desktop applications
- Re-download the credentials.json file

### Error: "access_denied" 
- Add your email as a test user if the app is in testing mode
- Make sure the Gmail API is enabled
- Check that you're signing in with the correct Google account

### Still not working?
- Delete `token.json` if it exists and try again
- Make sure you're using the correct `credentials.json` file
- Check that your project has the Gmail API enabled