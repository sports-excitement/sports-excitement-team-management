# Slack Time Tracker - Setup Guide

This is a comprehensive Slack time tracking system built with Go, featuring real-time monitoring of team members' working hours based on their Slack status.

## Features

- **Real-time Monitoring**: Track team members' working hours based on Slack status changes
- **Admin Dashboard**: Comprehensive dashboard with analytics and user activity
- **Data Export**: Export user data and weekly reports to Excel/CSV
- **WebSocket Updates**: Real-time dashboard updates
- **Slack Integration**: Uses Slack Socket Mode and Events API
- **Analytics**: Weekly and monthly hour tracking with progress visualization

## Prerequisites

- Go 1.21 or higher
- A Slack workspace with admin privileges
- Cloudflare Turnstile keys (optional, for enhanced security)

## Installation

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd sports-excitement-team-management
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Set up environment variables:**
   
   Copy `.env.example` to `.env` and fill in your values:
   ```bash
   cp .env.example .env
   ```

   Edit `.env` with your configuration:
   ```env
   SLACK_APP_TOKEN=xapp-your-app-token
   SLACK_BOT_TOKEN=xoxb-your-bot-token
   ADMIN_USER=admin
   ADMIN_PASS=your-secure-password
   TURNSTILE_SITE_KEY=your-turnstile-site-key
   TURNSTILE_SECRET_KEY=your-turnstile-secret-key
   PORT=3000
   DATABASE_PATH=./time_tracker.db
   ```

## Slack App Configuration

1. **Create a new Slack App:**
   - Go to https://api.slack.com/apps
   - Click "Create New App" → "From scratch"
   - Name your app and select your workspace

2. **Enable Socket Mode:**
   - Go to "Socket Mode" in the sidebar
   - Enable Socket Mode
   - Generate an App-Level Token with `connections:write` scope
   - Copy this token as your `SLACK_APP_TOKEN`

3. **Set Bot Token Scopes:**
   - Go to "OAuth & Permissions"
   - Add the following Bot Token Scopes:
     - `users:read`
     - `users:read.email`
   - Install the app to your workspace
   - Copy the Bot User OAuth Token as your `SLACK_BOT_TOKEN`

4. **Enable Events:**
   - Go to "Event Subscriptions"
   - Enable Events
   - Subscribe to bot events:
     - `user_change`
   - Save Changes

## Cloudflare Turnstile Setup (Optional)

1. **Get Turnstile Keys:**
   - Go to Cloudflare Dashboard → Turnstile
   - Create a new site
   - Copy the Site Key and Secret Key
   - Add them to your `.env` file

## Running the Application

1. **Start the server:**
   ```bash
   go run main.go
   ```

2. **Access the application:**
   - Open your browser and go to `http://localhost:3000`
   - You'll be redirected to the login page
   - Use the admin credentials you set in `.env`

## Usage

### Admin Dashboard

The dashboard provides:

- **Analytics Cards**: Total users, currently working, weekly/monthly hours
- **Status Distribution Chart**: Visual breakdown of working vs offline users
- **Weekly Progress Chart**: Individual user progress against 20-hour weekly target
- **User Activity Table**: Detailed user data with real-time updates

### Key Features

- **Real-time Updates**: Dashboard updates automatically when users change their Slack status
- **Status Detection**: Automatically detects "working" statuses based on keywords and emojis
- **Data Export**: Export user reports and weekly summaries to CSV/Excel
- **Responsive Design**: Works on desktop and mobile devices

### Working Status Detection

The system detects working status based on:

**Working Keywords:**
- working, coding, developing, programming
- busy, in a meeting, focus, focused
- deep work, heads down, do not disturb

**Working Emojis:**
- 💻 computer, laptop, keyboard
- 👨‍💻 technologist variations
- 🔧 tools and construction emojis

**Not Working Keywords:**
- lunch, break, away, out, offline
- vacation, sick, meeting, call
- commuting, traveling, afk

## Database

The application uses SQLite for data storage with the following main tables:

- **users**: Slack user information
- **time_entries**: Time tracking records
- **admins**: Admin user accounts
- **sessions**: User session data

## API Endpoints

- `GET /api/users` - Get user summaries
- `GET /api/analytics` - Get analytics data
- `GET /api/reports/weekly` - Get weekly reports
- `GET /api/export/excel` - Export data to CSV
- `GET /ws` - WebSocket connection for real-time updates

## Troubleshooting

### Common Issues

1. **Slack Connection Issues:**
   - Verify your Slack tokens are correct
   - Ensure Socket Mode is enabled
   - Check that the app is installed in your workspace

2. **Database Issues:**
   - The SQLite database is created automatically
   - Check file permissions in the application directory

3. **Authentication Issues:**
   - Verify admin credentials in `.env`
   - Clear browser cookies and try again

### Logs

The application logs important events to the console:
- Slack connection status
- User status changes
- Database operations
- WebSocket connections

## Development

To modify the application:

1. **Backend**: Edit files in `src/` directory
2. **Frontend**: Edit templates in `src/templates/` and assets in `public/`
3. **Database**: Modify models in `src/database/models.go`

The application uses:
- **Fiber**: Web framework
- **GORM**: ORM for database operations
- **Slack Go SDK**: Slack API integration
- **Bootstrap**: Frontend styling
- **Chart.js**: Data visualization
- **DataTables**: Table functionality

## Production Deployment

For production deployment:

1. **Build the application:**
   ```bash
   go build -o time-tracker main.go
   ```

2. **Use environment variables** instead of `.env` file

3. **Set up reverse proxy** (nginx/Apache) for HTTPS

4. **Use a proper database** (PostgreSQL/MySQL) for production

5. **Set up monitoring** and logging

## Support

For issues and questions:
1. Check the troubleshooting section
2. Review application logs
3. Verify Slack app configuration
4. Test with a minimal setup first 