# Gmail Label Hierarchy Fixer

A command-line tool to convert period-separated Gmail labels (like `Vacations.2025.Mexico`) into properly nested label hierarchies (`Vacations/2025/Mexico`). Perfect for cleaning up labels after IMAP migrations to Gmail.

## Features

- **Dry Run Analysis**: Analyze existing labels and preview changes without making modifications
- **Selective Fixes**: Fix specific labels individually 
- **Batch Processing**: Convert all period-separated labels at once
- **Safe Operations**: Preserves all email-to-label associations during conversion
- **Progress Tracking**: Real-time feedback for batch operations
- **Conflict Detection**: Identifies potential issues before making changes

## Installation

### Prerequisites

- Go 1.21 or higher
- Gmail API credentials (see Setup section)

### Build from Source

```bash
git clone <repository-url>
cd gmail-label-fixer
go build
```

## Setup

### Setup Google API Access

**📋 Quick Setup:** Follow the [OAuth Setup Guide](./setup-oauth.md) to:
1. Enable the Gmail API in Google Cloud Console
2. Create OAuth 2.0 credentials
3. Download the `credentials.json` file

### First Run Authentication

On first run, the tool will:
1. Automatically open your browser to Google's OAuth page
2. Ask you to sign in and grant permissions
3. Automatically redirect back and complete authentication
4. Save an authentication token (`token.json`) for future use

**Authentication Flow:**
```
🔐 Gmail Authentication Required
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🌐 Opening browser for secure authentication...
   URL: https://accounts.google.com/o/oauth2/auth?...

💡 This will open your browser and redirect back to this application
   securely. No manual code copying required!
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[Browser opens automatically]
✅ Authorization received!
🔄 Exchanging authorization code for access token...
✅ Authentication successful!
```

## Usage

### Analyze Labels (Dry Run)

Preview what changes would be made without actually modifying anything:

```bash
./gmail-label-fixer analyze
```

Example output:
```
🔍 Analyzing Gmail labels...
📊 Found 3 period-separated labels with 45 total messages

+------------------+----------------------+----------+------------------+
|  CURRENT LABEL   | NEW NESTED STRUCTURE | MESSAGES | REQUIRED PARENTS |
+------------------+----------------------+----------+------------------+
| Vacations.2025.Mexico | Vacations/2025/Mexico  |    12    |    2 parents     |
| Work.Projects.Q1      | Work/Projects/Q1       |    23    |    2 parents     |
| Archive.2024.Photos   | Archive/2024/Photos    |    10    |    2 parents     |
+------------------+----------------------+----------+------------------+

📁 Required parent labels to be created (4):
   - Archive/2024
   - Vacations/2025
   - Work/Projects
   - Work

💡 Next steps:
   - Fix specific label: gmail-label-fixer fix label --label "Vacations.2025.Mexico"
   - Fix all labels: gmail-label-fixer fix all
```

### Fix Specific Label

Convert a single period-separated label to nested hierarchy:

```bash
./gmail-label-fixer fix label --label "Vacations.2025.Mexico"
```

### Fix All Labels

Convert all detected period-separated labels:

```bash
./gmail-label-fixer fix all
```

Example output:
```
🔧 Fixing all period-separated labels...

[1/3] Processing: Vacations.2025.Mexico
   Creating parent label: Vacations
   Creating parent label: Vacations/2025
   Creating nested label: Vacations/2025/Mexico
   Moving 12 messages to new label...
   Deleting original label: Vacations.2025.Mexico
✅ Success: Vacations.2025.Mexico → Vacations/2025/Mexico

[2/3] Processing: Work.Projects.Q1
   Creating parent label: Work
   Creating parent label: Work/Projects
   Creating nested label: Work/Projects/Q1
   Moving 23 messages to new label...
   Deleting original label: Work.Projects.Q1
✅ Success: Work.Projects.Q1 → Work/Projects/Q1

🎉 Completed! Processed 3/3 labels successfully.
```

## How It Works

The tool performs the following steps for each period-separated label:

1. **Analysis**: Identifies labels containing periods and parses their hierarchy
2. **Parent Creation**: Creates any missing parent labels in the hierarchy
3. **Label Creation**: Creates the final nested label (e.g., `Vacations/2025/Mexico`)
4. **Message Migration**: Moves all emails from the old flat label to the new nested label
5. **Cleanup**: Deletes the original period-separated label

## Important Notes

- **Gmail API Limitations**: The Gmail API has rate limits. For large numbers of labels, the tool may need to pause between operations.
- **Backup Recommended**: While the tool preserves all emails, consider backing up your Gmail data before running batch operations.
- **Nested Structure**: Gmail uses forward slashes (`/`) to create nested label hierarchies. Labels with periods are treated as flat labels with periods in the name.

## Troubleshooting

### Authentication Issues

If you see authentication errors:
1. Follow the complete [OAuth Setup Guide](./setup-oauth.md)
2. Ensure `credentials.json` is in the correct location  
3. Delete `token.json` and re-authenticate
4. Verify your OAuth client is configured as "Desktop application"

### Rate Limiting

If you encounter rate limit errors:
1. Wait a few minutes and retry
2. Consider fixing labels in smaller batches using the selective fix option

### Conflicts

If the analysis shows conflicts (existing labels with the same names as targets):
1. Review the conflict list carefully
2. Manually resolve conflicts in Gmail before running the fix
3. Re-run analysis to verify conflicts are resolved

## Command Reference

```bash
# Analyze all labels (dry run)
./gmail-label-fixer analyze

# Fix specific label
./gmail-label-fixer fix label --label "Label.Name.Here"

# Fix all period-separated labels
./gmail-label-fixer fix all

# Show help
./gmail-label-fixer --help
```

## License

This tool is provided as-is for personal and educational use. Use at your own risk.