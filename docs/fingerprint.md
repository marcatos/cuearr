# Job fingerprint

Each split job is keyed by a fingerprint so enqueue stays idempotent for the same album inputs. The fingerprint is `sha256` of a pipe-separated payload: CUE path, image path, hex `sha256` of the CUE file bytes, image file size, image modification time (Unix nanoseconds), and hex `sha256` of the **full** lossless image file. If the image bytes change at the same path, the fingerprint changes and a new job is created even when an older job with the same paths already completed.
