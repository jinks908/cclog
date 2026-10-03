package domain

import (
	"testing"

	"github.com/annenpolka/cclog/internal/testutil"
)

func TestExtractCustomTitle(t *testing.T) {
	tests := []struct {
		name string
		log  *ConversationLog
		want string
	}{
		{
			name: "nil log",
			log:  nil,
			want: "",
		},
		{
			name: "empty log",
			log:  &ConversationLog{},
			want: "",
		},
		{
			name: "no custom-title message",
			log: &ConversationLog{Messages: []Message{
				{Type: "user", Message: map[string]any{"content": "Hello"}},
			}},
			want: "",
		},
		{
			name: "single custom-title message",
			log: &ConversationLog{Messages: []Message{
				{Type: "user", Message: map[string]any{"content": "Hello"}},
				{Type: "custom-title", CustomTitle: "my-session"},
			}},
			want: "my-session",
		},
		{
			name: "latest custom-title wins",
			log: &ConversationLog{Messages: []Message{
				{Type: "custom-title", CustomTitle: "old-name"},
				{Type: "user", Message: map[string]any{"content": "Hello"}},
				{Type: "custom-title", CustomTitle: "new-name"},
			}},
			want: "new-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.Diff(t, tt.want, ExtractCustomTitle(tt.log))
		})
	}
}
