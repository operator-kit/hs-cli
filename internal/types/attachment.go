package types

type Attachment struct {
	ID       int    `json:"id"`
	FileName string `json:"filename"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size,omitempty"`
}

type AttachmentCreate struct {
	FileName string `json:"filename"`
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}
