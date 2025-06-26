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

`go test -v ./pec`

## Parse PEC

Example how to read a PEC from `.eml` file and parse it:

```
// read email
email := ReadEmail("yourEmail.eml")

// parse email
untrustedPecMail, untrustedDatiCert, e := ParsePec(email)
if e != nil {
    log.Fatalf("failed to parse email: %v", e)
}
```

The result is an untrusted struct representing a PEC mail.

To verify the PEC signature use

```
// read email
email := ReadEmail("yourEmail.eml")

// verify email
if VerifySMIMEWithOpenSSL(email) != nil {
    log.Fatalf("failed to verify email")
}
```

