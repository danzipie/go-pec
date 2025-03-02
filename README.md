# go-pec
Posta Elettronica Certificata (PEC) in golang

In this project we parse the Italian Posta Elettronica Certificata (PEC) e-mail format.

The PEC MIME is organized as a S/MIME containing the message and two attachments: `daticert.xml` and the original email as an `.eml`:

```
multipart/mixed
├── multipart/alternative
│   ├── text/plain (PEC notification text)
│   ├── text/html (PEC notification in HTML format)
├── application/xml (daticert.xml)
├── message/rfc822 (original email as an .eml attachment)
├── application/pkcs7-signature (smime.p7s)
```

## Tests

`go test`

## Parse PEC

Example how to read a PEC from `.eml` file and parse it:

```
// read email
email := readEmail("yourEmail.eml")

// parse email
untrustedPecMail, untrustedDatiCert, e := parsePec(email)
if e != nil {
    log.Fatalf("failed to parse email: %v", e)
}
```

The result is an untrusted struct representing a PEC mail.

To verify the PEC signature use

```
// read email
email := readEmail("yourEmail.eml")

// verify email
if verifySMIMEWithOpenSSL(email) != nil {
    log.Fatalf("failed to verify email")
}
```

