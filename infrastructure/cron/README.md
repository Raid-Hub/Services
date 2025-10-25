# Cron Job Management

This directory contains cron job definitions for the RaidHub Services infrastructure.

## Structure

```
infrastructure/cron/
├── README.md           # This file
├── crontab/            # Crontab definitions by environment
│   ├── dev.crontab     # Development environment crontab
│   └── prod.crontab    # Production environment crontab
```

## Principles

### 1. No Hardcoded Credentials

- Never include passwords, API keys, or sensitive data in cron jobs
- All binaries automatically load environment variables from `.env` file
- Ensure `.env` file exists in the `RAIDHUB_PATH` directory

### 2. Proper Logging

- All cron jobs must log to `logs/cron-<job-name>.log`
- Log rotation should be configured in system cron
- All output is captured and logged automatically

### 3. Binary Loading of .env

All RaidHub binaries automatically load the `.env` file from their working directory using `godotenv.Load()`. This means:

- Jobs change to `$RAIDHUB_PATH` directory
- Binaries automatically find and load `.env` file
- No wrapper scripts needed

## Installation

### Production

1. Edit `prod.crontab` and update `RAIDHUB_PATH` to your actual production path
2. Ensure `.env` file exists in the production directory
3. Install the crontab:
   ```bash
   crontab infrastructure/cron/crontab/prod.crontab
   ```
4. Verify installation:
   ```bash
   crontab -l
   ```

### Development

Similar process using `dev.crontab`:

```bash
crontab infrastructure/cron/crontab/dev.crontab
```

## Environment Variables

The crontab sets up these environment variables:

- `RAIDHUB_ENV`: Environment (dev, prod)
- `RAIDHUB_PATH`: Path to RaidHub-Services directory (MUST be updated before installing)
- `PATH`: Standard system paths

All other environment variables (database credentials, API keys, etc.) are loaded from the `.env` file by the binaries.

## Production Jobs

### Database Backup

- **Frequency**: Daily at 2 AM
- **Command**: PostgreSQL backup script
- **Logs**: `logs/cron-db-backup.log`

### Log Cleanup

- **Frequency**: Weekly on Sunday at 3 AM
- **Command**: Clean old log files
- **Logs**: `logs/cron-log-cleanup.log`

### Metric Aggregation

- **Frequency**: Every 15 minutes
- **Command**: ClickHouse aggregation queries
- **Logs**: `logs/cron-metrics.log`

## Security

⚠️ **Important**: Never commit actual production crontabs with credentials!

1. Use environment variables for all sensitive data
2. Create `.env.production` file (git-ignored)
3. Document required environment variables in `example.env`
4. Use wrapper scripts to load environment variables

## Monitoring

Monitor cron job execution:

1. Check logs regularly: `tail -f logs/cron-*.log`
2. Set up alerts for failures
3. Monitor disk space for log files
4. Track execution time for performance

## Troubleshooting

### Job Not Running

- Check crontab is installed: `crontab -l`
- Check system cron service: `systemctl status cron`
- Check file permissions on scripts
- Check environment variables are loaded

### Permission Errors

- Ensure scripts are executable: `chmod +x infrastructure/cron/scripts/*`
- Check user has permission to write logs
- Check database credentials are correct

### Log Files Growing Too Large

- Implement log rotation in system cron
- Set up log cleanup jobs
- Monitor disk space
