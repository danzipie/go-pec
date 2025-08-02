CREATE TABLE pec_authorities (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    smtp_addr TEXT NOT NULL,
    notification_address TEXT NOT NULL
);

CREATE TABLE pec_cert_hashes (
    id SERIAL PRIMARY KEY,
    authority_id INTEGER NOT NULL REFERENCES pec_authorities(id),
    sha1_hash TEXT NOT NULL
);