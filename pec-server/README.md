
## Test

swaks --server localhost:1025 \
  --ehlo localhost \
  --auth \
  --auth-user username \
  --auth-password password \
  --from root@nsa.gov \
  --to root@gchq.gov.uk \
  --data @pec-server/sample.eml