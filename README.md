# Working Status Tracker

This is a Go-based application that uses HTMX and Fiber to create a simple dashboard for tracking the "Working" status of users in a Slack workspace.

## Features

- Simple admin authentication
- Dashboard to view user status changes
- Integration with Slack's Events API to listen for `user_change` events
- Uses SQLite for data storage

## Getting Started

### Prerequisites

- Go (version 1.18 or higher)
- A Slack workspace where you can create and install a custom app

### Installation

1.  **Clone the repository:**

    ```bash
    git clone https://github.com/sports-excitement/sports-excitement-team-management.git
    cd sports-excitement-team-management
    ```

2.  **Install the dependencies:**

    ```bash
    go mod tidy
    ```

### Configuration

1.  **Create a Slack App:**

    - Go to the [Slack API website](https://api.slack.com/apps) and create a new app.
    - From the **Features** menu, select **Socket Mode** and enable it.
    - Go to **OAuth & Permissions** and add the following Bot Token Scopes:
      - `users:read`
      - `users:read.email`
    - Go to **Event Subscriptions** and enable events.
    - In the **Subscribe to bot events** section, add the `user_change` event.
    - Install the app to your workspace.

2.  **Set up your environment variables:**

    - Rename the `.env.example` file to `.env`.
    - In the `.env` file, you'll need to add the following tokens:
      - `SLACK_APP_TOKEN`: This is the App-Level Token that starts with `xapp-`. You can generate this in the **Basic Information** section of your Slack app settings.
      - `SLACK_BOT_TOKEN`: This is the Bot User OAuth Token that starts with `xoxb-`. You can find this in the **OAuth & Permissions** section.

### Running the Application

1.  **Start the application:**

    ```bash
    go run main.go
    ```

2.  **Access the application:**

    - Open your web browser and go to `http://localhost:3000`.
    - You will be redirected to the login page.

## Usage

-   **Admin Credentials:**
    -   **Username:** `admin`
    -   **Password:** `admin`

-   **Dashboard:**
    -   After logging in, you will be taken to the dashboard, where you can view the status changes of users in your Slack workspace.

## How It Works

The application uses a custom Slack integration to track user status changes. Here's a brief overview of the technical approach:

-   **Slack Events API:** The application listens for the `user_change` event from the Slack Events API. This event is triggered whenever a user's profile information is updated, including their custom status.

-   **Timestamp Logging:** When a `user_change` event is received, the application checks if the user's status has been set to "Working" or changed from "Working" to something else. If it has, the application logs the timestamp of the status change in a SQLite database.

-   **Reporting:** The logged data can be used to calculate the total time each user spent with the "Working" status active over any given period and to generate reports as needed.

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change. 

## Logging System

The application now includes a comprehensive logging system with file-based storage and automatic rotation:

### Features
- **Dual Output**: Logs to both console and file (`./data/tracker.log`)
- **Log Levels**: INFO, ERROR, VERBOSE, and FATAL with conditional verbosity
- **Automatic Rotation**: Configurable size, backup count, and retention period
- **Compression**: Rotated logs are automatically compressed
- **API Management**: Endpoints for log statistics and manual rotation

### Configuration

Configure logging through environment variables in your `.env` file:

```bash
# Enable/disable verbose logging (default: true)
ENABLE_VERBOSE_LOGS=true

# Log file location (default: ./data/tracker.log)
LOG_FILE_PATH=./data/tracker.log

# Maximum log file size in MB before rotation (default: 10)
LOG_MAX_SIZE_MB=10

# Maximum number of backup files to keep (default: 5)
LOG_MAX_BACKUPS=5

# Maximum age in days to retain logs (default: 30)
LOG_MAX_AGE_DAYS=30
```

### Log Levels

- **INFO**: Important application events (always shown)
- **ERROR**: Error conditions (always shown) 
- **FATAL**: Critical errors that cause application exit (always shown)
- **VERBOSE**: Detailed debug information (only when `ENABLE_VERBOSE_LOGS=true`)

### API Endpoints

- `GET /api/logs/stats` - View log file statistics and configuration
- `POST /api/logs/rotate` - Manually trigger log rotation

### Storage Management

- **Automatic Rotation**: When log file reaches max size, it's rotated
- **Backup Naming**: `tracker.log.1`, `tracker.log.2`, etc.
- **Compression**: Old backups are gzipped to save space
- **Cleanup**: Old logs are automatically removed based on age and backup count
- **Directory Creation**: Log directory is created automatically if it doesn't exist

### Example Usage

```bash
# Production (minimal logging)
ENABLE_VERBOSE_LOGS=false

# Development (detailed logging)  
ENABLE_VERBOSE_LOGS=true

# Custom log location with smaller files
LOG_FILE_PATH=/var/log/tracker.log
LOG_MAX_SIZE_MB=5
LOG_MAX_BACKUPS=10
```

### Monitoring

Monitor your logs through:
- Direct file access: `tail -f data/tracker.log`
- API endpoint: `GET /api/logs/stats`
- Log rotation: `POST /api/logs/rotate`

The logging system ensures your application maintains detailed records while preventing disk space issues through intelligent rotation and cleanup. 