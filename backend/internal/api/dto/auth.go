package dto

type CredentialsRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"Password123!"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" example:"refresh-token"`
}

type AuthenticatedUser struct {
	ID    string `json:"id" example:"01kki6wb2vqqw4x63mw9bjw6y4"`
	Email string `json:"email" example:"user@example.com"`
}

type TokenPair struct {
	AccessToken           string `json:"access_token" example:"eyJhbGciOiJIUzI1NiJ9"`
	AccessTokenExpiresAt  string `json:"access_token_expires_at" example:"2026-03-12T16:00:00Z"`
	RefreshToken          string `json:"refresh_token" example:"opaque-refresh-token"`
	RefreshTokenExpiresAt string `json:"refresh_token_expires_at" example:"2026-04-11T16:00:00Z"`
}

type AuthResponse struct {
	User   AuthenticatedUser `json:"user"`
	Tokens TokenPair         `json:"tokens"`
}

type ErrorPayload struct {
	Code    string `json:"code" example:"invalid_request"`
	Message string `json:"message" example:"request body is invalid"`
}

type ErrorResponse struct {
	Error ErrorPayload `json:"error"`
}

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
	Time   string `json:"time" example:"2026-03-12T16:00:00Z"`
}
