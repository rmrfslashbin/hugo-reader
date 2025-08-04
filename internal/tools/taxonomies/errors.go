package taxonomies

// Error types for the taxonomies tool

// ErrInvalidRequest represents an error when the request is invalid
type ErrInvalidRequest struct {
	Err error
}

func (e *ErrInvalidRequest) Error() string {
	return e.Err.Error()
}

// ErrHugoSitePathRequired represents an error when the hugo_site_path is required
type ErrHugoSitePathRequired struct {
	Err error
}

func (e *ErrHugoSitePathRequired) Error() string {
	return "hugo_site_path is required"
}
