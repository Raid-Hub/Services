# Credential Management & Environment Configuration System

## Problem Statement

Currently, credentials, passwords, users, and permissions are scattered across:
- `.env` files (multiple environments)
- `docker-compose.yml` (hardcoded values, environment variable references)
- Individual service configurations
- Various configuration files

This creates:
- Security vulnerabilities (hardcoded passwords)
- Maintenance burden (update credentials in multiple places)
- Confusion about what credentials are used where
- Risk of credential leaks

## Proposed Solution

A unified credential management system that:

1. **Centralizes all credentials** in environment-specific configuration
2. **Eliminates hardcoded passwords** from all code and config files
3. **Provides clear environment separation** (dev, staging, prod)
4. **Uses secrets management** (Docker secrets, environment variables, or external secrets manager)
5. **Documents credential requirements** clearly
6. **Provides secure defaults** for local development

## Architecture

### Credential Hierarchy

```
Environment Credentials (high priority)
    ↓
Application Secrets (managed)
    ↓
Service Credentials (auto-generated or managed)
    ↓
Development Defaults (local only)
```

### Credential Categories

1. **Database Credentials**
   - PostgreSQL username/password
   - ClickHouse username/password
   - Connection strings

2. **Message Queue Credentials**
   - RabbitMQ username/password
   - Management UI credentials

3. **API Keys & Tokens**
   - Bungie API key
   - External service tokens

4. **Monitoring Credentials**
   - Prometheus credentials
   - Grafana admin credentials

5. **Service-to-Service Auth**
   - API authentication tokens
   - Service mesh certificates

6. **Tunnel Credentials**
   - Cloudflared tunnel credentials

## Implementation Plan

### Phase 1: Configuration Structure

#### Directory Structure

```
infrastructure/
├── credentials/               # Credential management
│   ├── templates/            # Template files
│   │   ├── .env.dev.template
│   │   ├── .env.staging.template
│   │   ├── .env.prod.template
│   │   └── .env.local.template
│   ├── generated/            # Auto-generated credentials
│   │   └── .gitignore        # Never commit these
│   └── README.md             # Credential management guide
├── docker/
│   ├── docker-compose.base.yml     # Base configuration
│   ├── docker-compose.dev.yml      # Dev overrides
│   ├── docker-compose.prod.yml     # Prod overrides
│   └── docker-compose.tilt.yml     # Tilt overrides
└── secrets/
    ├── docker-secrets/       # Docker secrets files
    └── README.md             # Secrets management
```

#### Environment File Organization

```bash
# Project root - GITIGNORED
.env                        # Auto-generated from template

# Infrastructure
infrastructure/credentials/
├── .env.dev.template       # Development defaults
├── .env.staging.template   # Staging defaults  
├── .env.prod.template      # Production defaults
└── .env.local.template     # Local overrides

# Generated files (never committed)
infrastructure/credentials/generated/
├── .env.dev                # Dev actual values
├── .env.staging            # Staging actual values
└── .env.prod               # Prod actual values
```

### Phase 2: Credential Generation

#### Credential Generation Script

```bash
infrastructure/credentials/generate.sh
```

Features:
- Generate secure random passwords
- Create environment-specific .env files
- Validate credential requirements
- Check for missing credentials
- Support for interactive setup
- Support for CI/CD integration

#### Usage

```bash
# Generate dev credentials
./infrastructure/credentials/generate.sh dev

# Generate staging credentials
./infrastructure/credentials/generate.sh staging

# Generate production credentials (interactive)
./infrastructure/credentials/generate.sh prod --interactive

# Validate credentials
./infrastructure/credentials/generate.sh validate

# Show credential requirements
./infrastructure/credentials/generate.sh requirements
```

### Phase 3: Docker Secrets Integration

#### Docker Compose with Secrets

```yaml
version: '3.8'

services:
  postgres:
    secrets:
      - db_password
      - db_root_password
    environment:
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
      POSTGRES_ROOT_PASSWORD_FILE: /run/secrets/db_root_password

secrets:
  db_password:
    file: ./infrastructure/secrets/docker-secrets/db_password.txt
  db_root_password:
    file: ./infrastructure/secrets/docker-secrets/db_root_password.txt
```

### Phase 4: Environment-Specific Configuration

#### Docker Compose Override Pattern

```bash
# Base configuration
docker-compose -f docker-compose.base.yml

# With environment-specific overrides
docker-compose \
  -f docker-compose.base.yml \
  -f docker-compose.dev.yml \
  --env-file .env

# Production
docker-compose \
  -f docker-compose.base.yml \
  -f docker-compose.prod.yml \
  --env-file .env.prod
```

## Credential Security

### Development Environment

- Default passwords (documented, easy to reset)
- No secrets required
- Quick setup for new developers

### Staging Environment

- Stronger defaults
- Secrets file required
- Mirrors production structure

### Production Environment

- External secrets manager (recommended)
- Docker secrets for Docker deployments
- Environment variables via CI/CD
- Never hardcode credentials
- Rotate credentials regularly

## Implementation Steps

### Step 1: Audit Current Credentials

- [ ] List all hardcoded passwords
- [ ] Document all credential locations
- [ ] Identify production vs development credentials
- [ ] List all environment variables

### Step 2: Create Credential Templates

- [ ] Create `.env.dev.template`
- [ ] Create `.env.staging.template`
- [ ] Create `.env.prod.template`
- [ ] Document all credential requirements

### Step 3: Build Generation Script

- [ ] Create credential generation script
- [ ] Implement password generation
- [ ] Add validation logic
- [ ] Add interactive mode

### Step 4: Refactor Docker Compose

- [ ] Split into base + environment files
- [ ] Remove hardcoded credentials
- [ ] Add secrets support
- [ ] Update environment variable references

### Step 5: Update Documentation

- [ ] Document credential management process
- [ ] Update setup instructions
- [ ] Create credential rotation procedures
- [ ] Add security best practices

### Step 6: Migrate Existing Credentials

- [ ] Generate new credentials
- [ ] Update all services
- [ ] Test in dev environment
- [ ] Deploy to staging
- [ ] Deploy to production

## Example Credential Template

### `.env.dev.template`

```bash
# Development Environment Configuration
# Copy this file to .env and customize as needed

# ============================================
# DATABASE CREDENTIALS
# ============================================
POSTGRES_USER=raidhub_dev
POSTGRES_PASSWORD=dev_password_change_me
POSTGRES_DB=raidhub_dev
POSTGRES_PORT=5432

CLICKHOUSE_USER=default
CLICKHOUSE_PASSWORD=
CLICKHOUSE_DB=raidhub

# ============================================
# MESSAGE QUEUE
# ============================================
RABBITMQ_USER=raidhub
RABBITMQ_PASSWORD=dev_rabbitmq_pass
RABBITMQ_MANAGEMENT_USER=admin
RABBITMQ_MANAGEMENT_PASSWORD=admin

# ============================================
# API KEYS
# ============================================
BUNGIE_API_KEY=your_api_key_here

# ============================================
# MONITORING
# ============================================
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=admin
PROMETHEUS_BASIC_AUTH_USER=prometheus
PROMETHEUS_BASIC_AUTH_PASSWORD=prometheus_pass

# ============================================
# CLOUDFLARE TUNNEL (Optional)
# ============================================
CLOUDFLARE_TUNNEL_TOKEN=
CLOUDFLARE_TUNNEL_URL=

# ============================================
# DEVELOPMENT SETTINGS
# ============================================
ENVIRONMENT=development
DEBUG=true
LOG_LEVEL=debug
```

## Security Best Practices

1. **Never commit credentials** to version control
2. **Use strong random passwords** in production
3. **Rotate credentials regularly** (every 90 days minimum)
4. **Use secrets managers** in production (AWS Secrets Manager, HashiCorp Vault, etc.)
5. **Limit credential scope** (principle of least privilege)
6. **Audit credential usage** regularly
7. **Encrypt credentials at rest** and in transit
8. **Use environment-specific credentials** (never share between environments)
9. **Document credential rotation procedures**
10. **Have incident response plan** for credential leaks

## Tools & Technologies

### Secrets Management Options

1. **Docker Secrets** (for Docker Swarm)
2. **Kubernetes Secrets** (for Kubernetes)
3. **HashiCorp Vault** (enterprise-grade)
4. **AWS Secrets Manager** (for AWS deployments)
5. **Environment Variables** (simple, secure with proper management)
6. **Sealed Secrets** (for GitOps)

### Credential Generation Tools

- `openssl rand` - Generate random passwords
- `pwgen` - Generate pronounceable passwords
- `diceware` - Passphrase generation
- `secret-keys` - Key generation utilities

## Migration Checklist

- [ ] Create credential templates
- [ ] Build generation script
- [ ] Refactor docker-compose files
- [ ] Update all service configurations
- [ ] Create setup documentation
- [ ] Test in development
- [ ] Test in staging
- [ ] Migrate production
- [ ] Update CI/CD pipelines
- [ ] Train team on new process
- [ ] Document credential rotation
- [ ] Set up monitoring/alerts

## Success Criteria

1. ✅ No hardcoded passwords in code
2. ✅ All credentials managed through configuration files
3. ✅ Environment-specific configurations working
4. ✅ Easy to generate new credentials
5. ✅ Clear documentation for credential management
6. ✅ Developers can set up local environment easily
7. ✅ Production credentials stored securely
8. ✅ Credential rotation process documented and tested
