package apierr

import (
	"fmt"
	"net/http"

	"github.com/its-aryansingh/qrit/services/internal/platform/httpx"
)

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ProblemDetails struct {
	Type     string       `json:"type"`
	Title    string       `json:"title"`
	Status   int          `json:"status"`
	Code     string       `json:"code"`
	Detail   string       `json:"detail"`
	Instance string       `json:"instance,omitempty"`
	Errors   []FieldError `json:"errors,omitempty"`
}

func (p *ProblemDetails) Error() string {
	return fmt.Sprintf("%s (%d): %s", p.Code, p.Status, p.Detail)
}

func New(status int, code, title, detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:   fmt.Sprintf("https://docs.example.com/errors/%s", code),
		Title:  title,
		Status: status,
		Code:   code,
		Detail: detail,
	}
}

func BadRequest(code, detail string, fieldErrors ...FieldError) *ProblemDetails {
	p := New(http.StatusBadRequest, code, "Bad Request", detail)
	p.Errors = fieldErrors
	return p
}

func Unauthorized(detail string) *ProblemDetails {
	return New(http.StatusUnauthorized, "unauthorized", "Unauthorized", detail)
}

func Forbidden(detail string) *ProblemDetails {
	return New(http.StatusForbidden, "forbidden", "Forbidden", detail)
}

func NotFound(detail string) *ProblemDetails {
	return New(http.StatusNotFound, "not_found", "Not Found", detail)
}

func Conflict(code, detail string) *ProblemDetails {
	return New(http.StatusConflict, code, "Conflict", detail)
}

func LimitReached(detail string) *ProblemDetails {
	return New(http.StatusPaymentRequired, "limit_reached", "Payment Required", detail)
}

func Internal(detail string) *ProblemDetails {
	return New(http.StatusInternalServerError, "internal_error", "Internal Server Error", detail)
}

func Render(w http.ResponseWriter, err error) {
	if prob, ok := err.(*ProblemDetails); ok {
		httpx.ProblemJSON(w, prob.Status, prob)
		return
	}
	prob := Internal("An unexpected internal error occurred.")
	httpx.ProblemJSON(w, prob.Status, prob)
}
