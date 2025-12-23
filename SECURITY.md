# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.3.x   | :white_check_mark: |
| 0.2.x   | :x:                |
| 0.1.x   | :x:                |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

1. **Do NOT** open a public GitHub issue for security vulnerabilities
2. Email your findings to: security@gabrielrauch.dev (or open a private security advisory on GitHub)
3. Include as much detail as possible:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

### What to Expect

- **Acknowledgment**: We will acknowledge receipt within 48 hours
- **Assessment**: We will assess the vulnerability and determine its severity
- **Fix Timeline**: Critical vulnerabilities will be addressed within 7 days
- **Disclosure**: We will coordinate disclosure timing with you

### Scope

The following are in scope for security reports:

- SQL injection vulnerabilities
- Path traversal attacks
- Authentication/authorization bypasses
- Remote code execution
- Denial of service (DoS)
- Information disclosure

### Out of Scope

- Issues in dependencies (please report to the upstream project)
- Issues requiring physical access
- Social engineering attacks
- Issues in third-party services

## Security Best Practices

When using Covenant in production:

1. **Use PostgreSQL or S3** for storage in production (not filesystem)
2. **Enable TLS** for broker connections
3. **Implement authentication** for the broker API
4. **Keep dependencies updated** using Dependabot
5. **Run security scans** in your CI/CD pipeline

## Security Updates

Security updates will be:

1. Released as patch versions (e.g., 0.1.1)
2. Documented in the CHANGELOG
3. Announced via GitHub releases

## Acknowledgments

We thank the following individuals for responsibly disclosing security issues:

- (Your name could be here!)
