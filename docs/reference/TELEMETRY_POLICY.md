# TaskWing Telemetry

TaskWing collects **anonymous usage statistics** to help improve the product. This document explains what data is collected, what is NOT collected, and how to opt out.

## What We Collect

| Data | Purpose |
|------|---------|
| Command name (e.g., `bootstrap`, `start`) | Understand which features are used |
| Command success/failure | Identify pain points |
| Error type (not details) | Prioritize bug fixes |
| Duration | Performance optimization |
| OS and architecture | Platform support prioritization |
| CLI version | Deprecation planning |

## What We DON'T Collect

- ❌ **Your code** or file contents
- ❌ **File paths** or project names
- ❌ **Search queries** or their content
- ❌ **API keys** or credentials
- ❌ **IP addresses** (not logged server-side)
- ❌ **Any personally identifiable information (PII)**

## How It Works

1. **First Run Consent**: On first use, TaskWing prompts you to enable/disable telemetry (opt-in, defaults to disabled)
2. **Anonymous ID**: A random UUID is generated (not tied to your identity)
3. **Local Storage**: Your preference is stored in `~/.taskwing/telemetry.json`
4. **Async & Non-blocking**: Telemetry never slows down your workflow
5. **CI Auto-Disable**: Telemetry is automatically disabled in CI/CD environments (GitHub Actions, GitLab CI, CircleCI, Travis, Jenkins, etc.)

## Opt Out

### Option 1: During Setup
When prompted on first run, press `n` to disable telemetry.

### Option 2: Via Command
```bash
taskwing config telemetry disable
```

### Option 3: Global Flag
```bash
taskwing --no-telemetry <command>
```

### Option 4: Check Status
```bash
taskwing config telemetry status
```

## Re-enable Telemetry

```bash
taskwing config telemetry enable
```

## Data Retention

- Telemetry data is retained for **90 days**
- Data is automatically deleted after this period
- We do not sell or share data with third parties

## GDPR Compliance

TaskWing telemetry is designed to be **GDPR-compliant by default**:
- Data is truly anonymous (no re-identification possible)
- Consent is explicit (opt-in via prompt)
- Easy opt-out at any time
- No cookies or persistent tracking

## Questions?

If you have questions about telemetry, please open an issue on [GitHub](https://github.com/josephgoksu/TaskWing).
