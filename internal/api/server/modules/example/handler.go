package example

import (
	"context"
	"errors"

	contract "github.com/supernurture/go-template/internal/api/server/oapicodegen/example"
)

// Handler only translates: generated request in, service call, generated response out.
// No validation and no dependency of its own, so the rules stay testable without HTTP.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

var _ contract.StrictServerInterface = (*Handler)(nil)

func (h *Handler) GetExampleVisits(
	ctx context.Context, _ contract.GetExampleVisitsRequestObject) (contract.GetExampleVisitsResponseObject, error) {
	visits, err := h.service.CountVisit(ctx)
	if err != nil {
		return nil, err
	}

	return contract.GetExampleVisits200JSONResponse{Visits: visits}, nil
}

func (h *Handler) ListExampleNotes(
	ctx context.Context, request contract.ListExampleNotesRequestObject) (contract.ListExampleNotesResponseObject, error) {
	notes, err := h.service.ListNotes(ctx, request.Params.Limit)
	if err != nil {
		// Returning the error unwrapped would make it a 500; only a caller mistake is a 400.
		if message, ok := validationMessage(err); ok {
			return contract.ListExampleNotes400JSONResponse{Message: message}, nil
		}
		return nil, err
	}

	response := make(contract.ListExampleNotes200JSONResponse, 0, len(notes))
	for _, stored := range notes {
		response = append(response, toContract(stored))
	}
	return response, nil
}

func (h *Handler) CreateExampleNote(
	ctx context.Context, request contract.CreateExampleNoteRequestObject) (contract.CreateExampleNoteResponseObject, error) {
	if request.Body == nil {
		return contract.CreateExampleNote400JSONResponse{Message: "a JSON body is required"}, nil
	}

	var body string
	if request.Body.Body != nil {
		body = *request.Body.Body
	}

	note, err := h.service.CreateNote(ctx, request.Body.Title, body)
	if err != nil {
		if message, ok := validationMessage(err); ok {
			return contract.CreateExampleNote400JSONResponse{Message: message}, nil
		}
		return nil, err
	}

	return contract.CreateExampleNote201JSONResponse(toContract(note)), nil
}

// validationMessage reports whether err is a caller mistake, and what to tell them.
func validationMessage(err error) (string, bool) {
	var invalid ValidationError
	if errors.As(err, &invalid) {
		return invalid.Message, true
	}
	return "", false
}

func toContract(stored Note) contract.Note {
	return contract.Note{
		Id:        stored.ID,
		Title:     stored.Title,
		Body:      stored.Body,
		CreatedAt: stored.CreatedAt,
	}
}
