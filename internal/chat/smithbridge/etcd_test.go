package smithbridge

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"smith/internal/source/model"
	"smith/internal/source/store"
)

func TestEtcdBridge_GetDocument(t *testing.T) {
	memStore := store.NewMemStore()
	bridge := NewEtcdBridge(memStore)
	ctx := context.Background()

	t.Run("document found", func(t *testing.T) {
		doc := model.Document{
			ID:        "doc-1",
			ProjectID: "proj-1",
			Title:     "Test Document",
			Content:   "This is a test document.",
			Format:    "text/plain",
			CreatedAt: time.Now().UTC(),
		}

		err := memStore.PutDocument(ctx, doc)
		assert.NoError(t, err)

		retrievedDoc, err := bridge.GetDocument(ctx, "doc-1")
		assert.NoError(t, err)
		assert.NotNil(t, retrievedDoc)

		assert.Equal(t, doc.ID, retrievedDoc.ID)
		assert.Equal(t, doc.ProjectID, retrievedDoc.ProjectID)
		assert.Equal(t, doc.Title, retrievedDoc.Title)
		assert.Equal(t, doc.Content, retrievedDoc.Content)
		assert.Equal(t, doc.Format, retrievedDoc.Format)
	})

	t.Run("document not found", func(t *testing.T) {
		retrievedDoc, err := bridge.GetDocument(ctx, "non-existent-doc")
		assert.Error(t, err)
		assert.Nil(t, retrievedDoc)
		assert.EqualError(t, err, "document not found")
	})
}
