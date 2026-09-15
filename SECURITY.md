# Security Policy

## Supported versions

RegLint is a CLI tool; only the latest release is supported with security fixes.

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Older releases | No |

## Reporting a vulnerability

Please report vulnerabilities privately through [GitHub private vulnerability reporting](https://github.com/reglint/reglint/security/advisories/new). Do not open a public issue for security reports.

Include the affected version (`reglint version`), a minimal reproduction, and the impact you observe. You can expect an initial response within 7 days.

## What RegLint does with your data

RegLint runs locally: it reads the files you point it at and writes findings only to the destinations you choose. It makes no network calls and uploads nothing.
