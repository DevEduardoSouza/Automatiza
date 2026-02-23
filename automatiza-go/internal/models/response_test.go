package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSuccessResponse(t *testing.T) {
	t.Parallel()

	t.Run("with string data", func(t *testing.T) {
		t.Parallel()
		resp := NewSuccessResponse("hello")
		assert.True(t, resp.Success)
		assert.Equal(t, "hello", resp.Data)
		assert.Empty(t, resp.Error)
		assert.Empty(t, resp.Message)
	})

	t.Run("with struct data", func(t *testing.T) {
		t.Parallel()
		type item struct {
			Name string
		}
		resp := NewSuccessResponse(item{Name: "test"})
		assert.True(t, resp.Success)
		assert.Equal(t, "test", resp.Data.Name)
	})
}

func TestNewSuccessResponseWithMessage(t *testing.T) {
	t.Parallel()

	resp := NewSuccessResponseWithMessage("data", "operation completed")
	assert.True(t, resp.Success)
	assert.Equal(t, "data", resp.Data)
	assert.Equal(t, "operation completed", resp.Message)
	assert.Empty(t, resp.Error)
}

func TestNewErrorResponse(t *testing.T) {
	t.Parallel()

	resp := NewErrorResponse("something went wrong")
	assert.False(t, resp.Success)
	assert.Equal(t, "something went wrong", resp.Error)
	assert.Nil(t, resp.Data)
}

func TestNewPaginatedResponse(t *testing.T) {
	t.Parallel()

	t.Run("with pagination", func(t *testing.T) {
		t.Parallel()
		p := &Pagination{Total: 100, Page: 1, Limit: 10, TotalPages: 10}
		resp := NewPaginatedResponse([]string{"a", "b"}, p)
		assert.True(t, resp.Success)
		assert.Equal(t, []string{"a", "b"}, resp.Data)
		assert.NotNil(t, resp.Pagination)
		assert.Equal(t, 100, resp.Pagination.Total)
		assert.Equal(t, 10, resp.Pagination.TotalPages)
	})

	t.Run("nil pagination", func(t *testing.T) {
		t.Parallel()
		resp := NewPaginatedResponse([]string{"x"}, nil)
		assert.True(t, resp.Success)
		assert.Nil(t, resp.Pagination)
	})
}
