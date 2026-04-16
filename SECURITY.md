# Security Policy

## Supported Versions

| Version | Supported |
|-------|----------|
| Latest (main) | ✅ |
| < 0.11.4 | ❌ |

Only the latest release receives security fixes. Update to the latest version.

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report security vulnerabilities through [GitHub Private Security Advisories](https://github.com/daptin/daptin/security/advisories/new).

Include:
- Description of the vulnerability and its impact
- Steps to reproduce (PoC or proof of concept preferred)
- Affected versions
- Suggested fix if you have one

You will receive a response within **72 hours**. If the vulnerability is accepted, a fix will be prioritized and a CVE requested. You will be credited in the advisory and release notes unless you prefer otherwise.

## Security Advisories

Published advisories are listed at:
https://github.com/daptin/daptin/security/advisories

| Advisory | CVE | Severity | Fixed In | Summary |
|----------|-----|---------|---------|--------|
| [GHSA-rw2c-8rfq-gwfv](https://github.com/daptin/daptin/security/advisories/GHSA-rw2c-8rfq-gwfv) | Pending | High (8.3) | v0.11.4 | SQL injection via unvalidated `goqu.L()` calls in aggregate API (CWE-89) |

## Scope

The following are in scope for security reports:

- SQL injection / NoSQL injection
- Authentication or authorization bypass
- Remote code execution
- Sensitive data exposure
- Server-side request forgery (SSRF)
- Path traversal
- Denial of service via panic or resource exhaustion

The following are out of scope:

- Vulnerabilities in dependencies (report those upstream)
- Issues requiring physical access to the server
- Social engineering
