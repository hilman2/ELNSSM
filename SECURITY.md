# Security Policy

## Supported versions

Until ELNSSM reaches `v1.0.0`, only the latest tagged release receives
security fixes. After `v1.0.0`, the policy will be updated to cover the latest
minor release line.

| Version | Supported |
| --- | --- |
| latest `v0.x` | yes |
| older `v0.x` | no |

## Reporting a vulnerability

**Please do not open a public GitHub issue for security problems.**

Instead, use GitHub's private vulnerability reporting:

1. Go to <https://github.com/hilman2/ELNSSM/security/advisories>
2. Click **Report a vulnerability**
3. Fill in the form with as much detail as possible

If you cannot use GitHub for some reason, contact the maintainer listed in
the repository profile.

Please include:

- A description of the issue and its impact
- Steps to reproduce or a proof of concept
- Affected version(s) and platform
- Any suggested mitigation

We aim to acknowledge new reports within **72 hours** and to provide an
initial assessment within **7 days**.

## Disclosure process

1. We confirm the report and start working on a fix in a private branch.
2. We coordinate a release date with the reporter.
3. A patched release is published, followed by a public security advisory
   crediting the reporter (unless anonymity is requested).

## Scope

In scope:

- The `elnssm` binary, including the Guardian service, REST/WebSocket API and
  embedded web GUI
- Default configuration as shipped in `configs/`

Out of scope:

- Vulnerabilities in third-party dependencies (please report those upstream;
  we will pull in fixes once available)
- Issues that require local administrator privileges already equivalent to
  what the Guardian itself runs with

Thank you for helping keep ELNSSM and its users safe.
