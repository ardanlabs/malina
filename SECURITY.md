# Security Policy

## Reporting a vulnerability

Please do not open a public issue for a suspected vulnerability. Use this
repository's **Security** tab and GitHub private vulnerability reporting to
send the maintainers a report. Include affected versions, reproduction steps,
impact, and any suggested mitigation. The maintainers will coordinate
validation and disclosure through that private report.

## Supported versions

Malina is evolving and does not currently promise a fixed long-term-support
window. Security fixes are made on the default branch and, when practical, in
the latest released version. Users should run the newest available release and
check release notes when upgrading. Older releases may not receive backports.

## Trust boundaries

Malina dynamically loads native `stable-diffusion.cpp` code. `MALINA_LIB` (or
the equivalent CLI option) controls where that code is loaded from. A native
bundle executes with the privileges of the Malina process, and model files are
complex, externally supplied inputs processed by native code. Treat both
libraries and models as executable-content trust boundaries: obtain them from
trusted sources, restrict who can replace them, and avoid loading files from
untrusted users.

The library installer obtains release artifacts originating from the upstream
[`leejet/stable-diffusion.cpp`](https://github.com/leejet/stable-diffusion.cpp)
project. Upstream provenance is not the same as a project-specific security
audit, and checksum/signature availability may vary by upstream release.
Verify artifact provenance and checksums where your deployment requires it.

## Network deployment

The HTTP API and administration BUI currently provide neither authentication
nor TLS. They should not be exposed directly to the public internet or an
untrusted network. Bind to loopback or a trusted private interface, apply host
firewall rules, and put an authenticated, TLS-terminating reverse proxy in
front when remote access is necessary. Restrict access to model-management
operations, apply request/body and resource limits at the proxy, and run the
process as a dedicated least-privileged account with narrowly writable model
and library directories.
