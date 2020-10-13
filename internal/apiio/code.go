package apiio

type RunCodeRequest struct {
	Code      string `json:"code,omitempty"`
	Container string `json:"container,omitempty"`
}
