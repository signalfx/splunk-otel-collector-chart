# Example of chart configuration

## Use existing secret

This example shows how reuse existing secret with o11y token. In this case we use the default
`serviceAccount` settings and assume it is enough to access the `secret`.

Sometimes we might need to use existing `serviceAccount` to be able to read the secret, then
the configuration must be:

```yaml
serviceAccount:
  create: false
  name: name-of-existing-sa
```
