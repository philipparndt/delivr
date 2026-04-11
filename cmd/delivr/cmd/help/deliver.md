Upload metadata and screenshots to App Store Connect.

## Usage

```bash
delivr deliver --config <appstore.yaml> [options]
```

## Authentication

Credentials can be passed via flags or environment variables:

| Flag          | Env Variable    | Description                       |
|---------------|-----------------|-----------------------------------|
| `--key-id`    | `ASC_KEY_ID`    | App Store Connect API Key ID      |
| `--issuer-id` | `ASC_ISSUER_ID` | App Store Connect Issuer ID       |
| `--key-file`  | `ASC_KEY_FILE`  | Path to `.p8` private key file    |
| `--key-pem`   | `ASC_KEY_PEM`   | Inline PEM private key content    |

## Options

Use `--skip-metadata` to upload only screenshots, or `--skip-screenshots`
to upload only metadata.

## Subcommands

| Command              | Description                              |
|----------------------|------------------------------------------|
| `list-display-types` | List available display types for the app |

## Examples

```bash
delivr deliver --config appstore.yaml
delivr deliver --config appstore.yaml --skip-metadata
delivr deliver list-display-types --config appstore.yaml
```
