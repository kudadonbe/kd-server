# Identity Documents

KD-Server stores identity-document facts separately from the unified person record. The first supported profile is the Maldivian national identity card.

## Stored fields

- Tenant, KD person ID, document ID, document type, and country
- National ID and document serial number
- English and Dhivehi names and common names
- Sex, date of birth, address, blood group, and expiry date
- Signature and fingerprint presence flags
- Source, extraction method, per-field confidence, and verification status
- Optional secured references to front and back images
- Version, creation time, and update time

The server stores structured facts, not HTML card markup. The admin UI renders those facts as a front/back preview.

## Local document extraction

The admin console accepts PDF, JPEG, and PNG copies up to 10 MB. Extraction uses local Tesseract OCR and Poppler:

1. The upload is copied into a randomly named temporary directory with owner-only permissions.
2. PDFs are converted to JPEG images. Only the first two pages are processed.
3. Tesseract runs with English and Dhivehi language data.
4. Suggested fields, raw OCR text, warnings, and confidence values are returned to the browser.
5. The temporary directory and uploaded copy are deleted immediately.
6. Staff must compare the suggestions with the original document and explicitly save the reviewed record.

Extraction does not automatically create or update an identity-document record. The raw OCR text is not stored by the extraction endpoint.

Required local tools:

- Tesseract OCR 5 with `eng` and `div` language data
- Poppler `pdftoppm` for PDF files

## Trace history

Every create or update writes an immutable snapshot to `identity_document_history`. History entries include the tenant, person, document, version, event type, actor, and timestamp.

## Security

- Identity documents are tenant scoped.
- Do not place identity values in application logs.
- Store only secured image references; do not expose filesystem paths or public object URLs.
- Production deployments should use encrypted storage, access auditing, retention rules, and least-privilege credentials.
- The admin card is marked as a data preview and must not be treated as an official credential.
- Do not log uploaded documents, OCR text, extracted identity values, or image contents.
