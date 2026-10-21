package ocr

import _ "embed"

const Attribution = "Indra OCR 1.1.2 by Datalab (Apache-2.0 code). Unmodified Datalab models; separate modified OpenRAIL-M terms apply to models and outputs."
const ModelLicenseURL = "https://huggingface.co/datalab-to/indra-ocr/blob/bd02167fdf2005a19962cc15188f8496a2c87d3b/LICENSE"

//go:embed runtime/LICENSE-MODELS.txt
var ModelLicense []byte
