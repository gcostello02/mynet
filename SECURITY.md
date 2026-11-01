# Security policy

## Reporting a vulnerability

Please **don't open a public issue** for security problems.

Report them privately through GitHub: go to the repository's **Security** tab and choose **Report a vulnerability**
([direct link](https://github.com/gcostello02/mynet/security/advisories/new)). Include what you found, how to reproduce it,
and which version or commit you tested.

You can expect an acknowledgement within a few days. This is a small personal project, so fixes are best-effort, but
credible reports about enrollment, authentication, key handling or the admin API will be prioritized.

## Scope

In scope: the coordinator (`mynet-server`), the `mynet` client and daemon, the embedded admin dashboard, and the deployment files in this repository.

Out of scope: vulnerabilities in third-party dependencies (report those upstream), and problems that require an attacker to already control your coordinator host or an enrolled device's root account.

## Design and hardening notes

How enrollment, keys and revocation work, and the known limitations, are described in [docs/security.md](docs/security.md).
