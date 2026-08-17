package example

import (
	"context"
	"errors"
	"testing"
	"time"

	contract "github.com/supernurture/go-template/internal/api/server/oapicodegen/example"
)

func nilHandler() *Handler { return NewHandler(nilService()) }

// The spec documents a 400 for a caller mistake; anything else has to stay a 500, so
// the handler must not blanket-convert errors.
func TestHandlerMapsValidationErrorsTo400(t *testing.T) {
	handler := nilHandler()

	t.Run("create", func(t *testing.T) {
		request := contract.CreateExampleNoteRequestObject{Body: &contract.CreateNoteRequest{Title: "  "}}

		response, err := handler.CreateExampleNote(context.Background(), request)
		if err != nil {
			t.Fatalf("CreateExampleNote: %v", err)
		}
		bad, ok := response.(contract.CreateExampleNote400JSONResponse)
		if !ok {
			t.Fatalf("response = %T, want a 400", response)
		}
		if bad.Message != "title is required" {
			t.Errorf("message = %q, want the service's message", bad.Message)
		}
	})

	t.Run("list", func(t *testing.T) {
		limit := maxLimit + 1
		request := contract.ListExampleNotesRequestObject{Params: contract.ListExampleNotesParams{Limit: &limit}}

		response, err := handler.ListExampleNotes(context.Background(), request)
		if err != nil {
			t.Fatalf("ListExampleNotes: %v", err)
		}
		if _, ok := response.(contract.ListExampleNotes400JSONResponse); !ok {
			t.Errorf("response = %T, want a 400", response)
		}
	})
}

// A missing body never reaches the service, so the handler answers it itself.
func TestCreateWithoutBodyIs400(t *testing.T) {
	response, err := nilHandler().CreateExampleNote(context.Background(), contract.CreateExampleNoteRequestObject{})
	if err != nil {
		t.Fatalf("CreateExampleNote: %v", err)
	}

	bad, ok := response.(contract.CreateExampleNote400JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want a 400", response)
	}
	if bad.Message != "a JSON body is required" {
		t.Errorf("message = %q", bad.Message)
	}
}

// A dependency failure is not the caller's fault: it must surface as an error so the
// generated server answers 500, not as a 400.
func TestHandlerPassesRealFailuresThrough(t *testing.T) {
	repo, mock := mockRepository(t)
	mock.ExpectQuery(`SELECT \* FROM "example_notes"`).WillReturnError(errors.New("connection reset"))

	handler := NewHandler(NewService(nil, repo))
	response, err := handler.ListExampleNotes(context.Background(), contract.ListExampleNotesRequestObject{})

	if err == nil {
		t.Fatalf("response = %v, want the failure to surface as an error", response)
	}
	var invalid ValidationError
	if errors.As(err, &invalid) {
		t.Error("a database failure was reported as a caller mistake")
	}
}

func TestGetExampleVisits(t *testing.T) {
	service, server := miniredisService(t)
	handler := NewHandler(service)

	response, err := handler.GetExampleVisits(context.Background(), contract.GetExampleVisitsRequestObject{})
	if err != nil {
		t.Fatalf("GetExampleVisits: %v", err)
	}
	ok, isOK := response.(contract.GetExampleVisits200JSONResponse)
	if !isOK {
		t.Fatalf("response = %T, want a 200", response)
	}
	if ok.Visits != 1 {
		t.Errorf("visits = %d, want 1", ok.Visits)
	}

	// Redis being down is not the caller's fault, so it must not become a 400.
	server.Close()
	if _, err := handler.GetExampleVisits(context.Background(), contract.GetExampleVisitsRequestObject{}); err == nil {
		t.Error("expected the Redis failure to surface as an error")
	}
}

func TestCreateExampleNoteReturns201(t *testing.T) {
	repo, mock := mockRepository(t)
	expectInsert(mock, 3)

	handler := NewHandler(NewService(nil, repo))
	request := contract.CreateExampleNoteRequestObject{
		Body: &contract.CreateNoteRequest{Title: "First note", Body: new("the body")},
	}

	response, err := handler.CreateExampleNote(context.Background(), request)
	if err != nil {
		t.Fatalf("CreateExampleNote: %v", err)
	}

	created, ok := response.(contract.CreateExampleNote201JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want a 201", response)
	}
	if created.Id != 3 || created.Title != "First note" || created.Body != "the body" {
		t.Errorf("note = %+v, want the stored values", created)
	}
}

// A create that fails in the database must stay an error, not a 201 or a 400.
func TestCreateExampleNotePassesRealFailuresThrough(t *testing.T) {
	repo, mock := mockRepository(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "example_notes"`).WillReturnError(errors.New("disk full"))
	mock.ExpectRollback()

	handler := NewHandler(NewService(nil, repo))
	request := contract.CreateExampleNoteRequestObject{Body: &contract.CreateNoteRequest{Title: "First note"}}

	if _, err := handler.CreateExampleNote(context.Background(), request); err == nil {
		t.Fatal("expected the insert failure to surface as an error")
	}
}

func TestListExampleNotesReturnsEveryRow(t *testing.T) {
	repo, mock := mockRepository(t)
	created := time.Date(2026, 8, 17, 10, 30, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT \* FROM "example_notes"`).
		WillReturnRows(mock.NewRows([]string{"id", "title", "body", "created_at"}).
			AddRow(int64(2), "newer", "", created).
			AddRow(int64(1), "older", "", created))

	handler := NewHandler(NewService(nil, repo))
	response, err := handler.ListExampleNotes(context.Background(), contract.ListExampleNotesRequestObject{})
	if err != nil {
		t.Fatalf("ListExampleNotes: %v", err)
	}

	notes, ok := response.(contract.ListExampleNotes200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want a 200", response)
	}
	if len(notes) != 2 || notes[0].Title != "newer" || notes[1].Id != 1 {
		t.Errorf("notes = %+v, want both rows in order", notes)
	}
}

// An empty table must marshal as [] rather than null, which breaks strict clients.
func TestListExampleNotesWithNoRows(t *testing.T) {
	repo, mock := mockRepository(t)
	mock.ExpectQuery(`SELECT \* FROM "example_notes"`).
		WillReturnRows(mock.NewRows([]string{"id", "title", "body", "created_at"}))

	handler := NewHandler(NewService(nil, repo))
	response, err := handler.ListExampleNotes(context.Background(), contract.ListExampleNotesRequestObject{})
	if err != nil {
		t.Fatalf("ListExampleNotes: %v", err)
	}

	notes, ok := response.(contract.ListExampleNotes200JSONResponse)
	if !ok || notes == nil {
		t.Fatalf("response = %#v, want an empty non-nil slice", response)
	}
	if len(notes) != 0 {
		t.Errorf("notes = %+v, want none", notes)
	}
}

// Adjacent string fields: a swap here would return the wrong data and still compile.
func TestToContract(t *testing.T) {
	created := time.Date(2026, 8, 17, 10, 30, 0, 0, time.UTC)
	got := toContract(Note{ID: 7, Title: "the title", Body: "the body", CreatedAt: created})

	want := contract.Note{Id: 7, Title: "the title", Body: "the body", CreatedAt: created}
	if got != want {
		t.Errorf("toContract = %+v, want %+v", got, want)
	}
}
