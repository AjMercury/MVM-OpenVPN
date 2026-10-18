> Edition update — 2026-10-19: Both editions retain AES-256-GCM. Native Linux reads the unchanged key through protected MVM_MASTER_KEY_FILE, outside SQLite. See [migration/key custody](../operations/edition-migration.md); never generate a new key for an existing database.

# Secrets and PKI

Generate `MVM_MASTER_KEY` from 32 random bytes, encoded with standard base64. Save it in an access-controlled secret manager and a separately protected recovery copy. Never save it inside the database, image, source or backup. Loss of the key means loss of encrypted application records. A fresh key is not a recovery method.

All repository values are sealed using AES-GCM. Router passwords, account hashes, job inputs, session data and temporary delivery profiles are encrypted at rest. File permissions are 0600 and parent directories 0700 on systems that implement Unix modes; Windows requires appropriate ACLs. Keep swap, crash dumps, endpoint backups and administrator access within your security boundary.

Router-held CA keys must be included in a separately authorized encrypted RouterOS backup with tested restore procedures. An app database backup alone cannot restore those keys. Do not export the CA private key for client issuance. Client certificates have `tls-client`; the server has `tls-server`, digital signature and key encipherment. Common Name/SAN are derived from validated input. The implementation uses RSA 2048-bit keys and a 365-day leaf/3650-day CA request.

Certificate export produces files on RouterOS; export success is not a file download. The backend reads `/file/read` in chunks ≤32768 bytes, checks exact file size and refuses partial content. It retrieves only the CA certificate, client certificate and encrypted client key; it never requests CA/server private keys. The certificate chain, client EKU and certificate/private-key association are validated before profile generation. Both traditional encrypted RSA PEM and PBES2 encrypted PKCS#8 are parsed with bounded KDF work. The backend transiently decrypts the exported client key using the provided passphrase to check the public-key match, but delivers the original encrypted bytes and never stores the passphrase. The VPN client still prompts for the same passphrase after delivery. Actual RouterOS export and client import require live testing.

Temporary export names have a random prefix. An export intent with that prefix is persisted before any export command, without the passphrase. Cleanup runs on ordinary error paths, but a process crash can leave exported files. Restart flags the interrupted export job for manual intervention and blocks new provisioning. Review only its exact `mvm-export-` names in router `/file`; do not indiscriminately remove existing files. The recovery runbook must be followed before resuming provisioning. Credentials should not be delivered through public chat or email without an authorized secure process.

Key rotation is presently a **maintenance procedure**, not a shipped API: stop the service, make/verify a backup, decrypt and re-encrypt every record in an independently reviewed migration tool, validate the new database, and retain the old database/key pair until recovery is tested. No automated rotation tool is included. Do not merely replace the environment key.

Certificate revoke/reissue/rotation, CRL distribution, expiration enforcement while offline and tested CN-to-PPP binding are pending. Disable credentials **and terminate sessions** for the implemented offboarding path; check reconnection fails in the lab.

