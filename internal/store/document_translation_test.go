package store

import (
	"context"
	"strings"
	"testing"
)

func TestSQLiteDocumentTranslationFields(t *testing.T) {
	database, err := OpenSQLite(context.Background(), t.TempDir()+"/hohoho.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	document, err := database.CreateDocument(context.Background(), CreateDocument{
		OriginalFilename: "source.txt",
		MIMEType:         "text/plain",
		SizeBytes:        5,
		SHA256:           strings.Repeat("a", 64),
		ObjectKey:        strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if document.TranslationStatus != "not_requested" || document.TranslationText != "" {
		t.Fatalf("unexpected initial translation: %+v", document)
	}
	if err := database.UpdateDocumentTranslation(context.Background(), document.ID, "completed", "Hallo"); err != nil {
		t.Fatal(err)
	}
	document, err = database.Document(context.Background(), document.ID)
	if err != nil {
		t.Fatal(err)
	}
	if document.TranslationStatus != "completed" || document.TranslationText != "Hallo" {
		t.Fatalf("unexpected stored translation: %+v", document)
	}
}
