# Terraform

## Overview

AWS infrastructure is managed with Terraform in the `aws/` directory. The
setup covers DNS for 15 domains, SES email sending, a CloudFront CDN, IAM
credentials for Go apps, and remote state storage. Domains are registered
on Gandi; Terraform points their nameservers to Route53.

## Directory Structure

```
aws/
├── .envrc                      # AWS credentials (direnv)
├── convert-zones.fish          # Zone file → Terraform code generator
├── tasks.toml                  # Task runner definitions
├── tfz53-macos / tfz53-linux   # Zone-to-Terraform converter binaries
├── zones/                      # Bind-format zone files (source of truth for DNS)
│   ├── monks.co
│   ├── ss.cx
│   └── ... (15 domains)
└── terraform/
    ├── main.tf                 # Providers, backend, module instantiation, outputs
    ├── gandi.tf                # Gandi nameserver delegation for all domains
    ├── iam.tf                  # IAM user, policies, access keys
    ├── tfstate.tf              # S3 bucket + DynamoDB table for state
    ├── x.monks.co.tf           # public_bucket module for x.monks.co CDN
    ├── generated_*.tf          # Auto-generated from zone files (do not edit)
    ├── mailer/                 # Module: SES email for a domain
    └── public_bucket/          # Module: S3 + CloudFront static site
```

## DNS Management

DNS records are authored as standard bind zone files in `aws/zones/`.
A `convert-zones.fish` script runs `tfz53` (a prebuilt binary) to convert
each zone file into a `generated_<domain>.tf` file containing
`aws_route53_zone` and `aws_route53_record` resources. These generated
files are committed to the repo but should not be hand-edited.

To add or change a DNS record, edit the zone file and run the
`aws-convert-zones` task (or `plan`/`apply`, which depend on it).

### Domains

15 domains are managed: monks.co, ss.cx, amonks.co, andrewmonks.com,
andrewmonks.net, andrewmonks.org, belgianman.com, blgn.mn, docrimes.com,
fmail.email, fuckedcars.com, lyrics.gy, needsyourhelp.org,
piano.computer, popefucker.com.

### Nameserver Delegation

`gandi.tf` uses the `go-gandi/gandi` provider to set each domain's
nameservers on the Gandi registrar to the Route53 nameservers assigned
to the corresponding hosted zone. This requires a
`GANDI_PERSONAL_ACCESS_TOKEN` Terraform variable.

## Modules

### `mailer/`

Sets up SES email sending for a domain. Creates a per-domain IAM user
with a policy allowing `ses:SendRawEmail`, configures SES domain identity
verification via DNS, generates DKIM keys and corresponding Route53
records, and adds MX/SPF/TXT records. Outputs SMTP username and password.

Currently instantiated for ss.cx only.

### `public_bucket/`

Creates an S3 bucket with public read access, a CloudFront distribution
with an ACM TLS certificate (validated via DNS), and a Route53 A record
pointing to the CloudFront distribution.

Currently instantiated for x.monks.co only.

## IAM

A single IAM user `monks-go` is created for use by Go apps. It has two
policies attached:

- **write_dns_records_for_acme_challenge**: Route53 read/write access
  for ACME DNS-01 challenges (used by `pkg/tls` for TLS cert
  provisioning via CertMagic).
- **send_ss_cx_emails**: SES `SendRawEmail` for `no-reply@mail.ss.cx`
  (used by `apps/mailer`).

The IAM access key ID and secret are exposed as Terraform outputs.

## State

Terraform state is stored remotely in S3 with DynamoDB locking. The
resources that hold the state are themselves managed by Terraform in
`tfstate.tf`:

- **S3 bucket** `monks-co-tfstate` (us-east-1): versioned, AES256
  server-side encryption.
- **DynamoDB table** `monks-co-tfstate-lock`: PAY_PER_REQUEST billing,
  `LockID` hash key.

## Providers

| Provider | Version | Purpose |
|----------|---------|---------|
| hashicorp/aws | 5.74.0 | Route53, S3, CloudFront, IAM, SES, ACM, DynamoDB |
| go-gandi/gandi | >= 2.1.0 | Nameserver delegation at the registrar |

## Tasks

Run from the repo root via the task runner. The root `tasks.toml`
exposes a top-level `terraform-apply` task that delegates to `aws/apply`.

| Task | Purpose |
|------|---------|
| `aws-convert-zones` | Regenerate `generated_*.tf` from zone files |
| `fmt` | Format Terraform files (depends on convert-zones) |
| `init` | `terraform init` |
| `plan` | `terraform plan` (depends on convert-zones) |
| `apply` | `terraform apply` with auto-approve (depends on convert-zones) |

All Terraform commands source `.envrc` for AWS credentials and run from
the `terraform/` subdirectory.
