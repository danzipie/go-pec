
# Install

Generate a PEC trusted authority certificate:

```openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem \
  -days 365 -nodes -subj "/C=IT/O=PEC Test/CN=posta-certificata.local" \
  -addext "extendedKeyUsage=emailProtection" \
  -addext "subjectAltName=email:posta-certificata@localhost"
```

## Test

swaks --server localhost:1025 \
  --ehlo localhost \
  --auth \
  --auth-user username \
  --auth-password password \
  --from root@nsa.gov \
  --to root@gchq.gov.uk \
  --data @pec-server/sample.eml