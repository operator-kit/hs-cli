package types

type PageInfo struct {
	Number        int `json:"number"`
	Size          int `json:"size"`
	TotalElements int `json:"totalElements"`
	TotalPages    int `json:"totalPages"`
}

type DocsPageInfo struct {
	Page  int `json:"page"`
	Pages int `json:"pages"`
	Count int `json:"count"`
}
